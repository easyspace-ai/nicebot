package bot

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"limitorderbot/internal/chain"
	"limitorderbot/internal/clob"
	"limitorderbot/internal/logging"
	"limitorderbot/internal/models"
)

func (b *Bot) mergePositionsIfPossible(ctx context.Context, market models.Market, orders []models.OrderRecord) float64 {
	yesToken, noToken := inferYesNoTokenIDs(market, orders)
	if yesToken == "" || noToken == "" {
		return 0
	}

	yesBal, err := b.chain.ERC1155BalanceOf(ctx, common.HexToAddress(chain.CTFAddress), mustBigInt(yesToken))
	if err != nil {
		return 0
	}
	noBal, err := b.chain.ERC1155BalanceOf(ctx, common.HexToAddress(chain.CTFAddress), mustBigInt(noToken))
	if err != nil {
		return 0
	}

	yes := toFloat6(yesBal)
	no := toFloat6(noBal)
	if yes <= 0 || no <= 0 {
		return 0
	}
	mergeable := math.Min(yes, no)
	already := b.mergedAmounts[market.ConditionID]
	mergeAmt := mergeable - already
	if mergeAmt <= 0.001 {
		return 0
	}

	cid, err := chain.ConditionIDFromHex(market.ConditionID)
	if err != nil {
		return 0
	}
	tx, err := b.chain.MergePositions(ctx, cid, big.NewInt(int64(mergeAmt*1e6)))
	if err != nil {
		logging.Logger().Printf("Merge failed: %v\n", err)
		return 0
	}
	logging.Logger().Printf("Merged %.6f sets for %s (tx=%s)\n", mergeAmt, market.MarketSlug, tx.Hex())
	b.mergedAmounts[market.ConditionID] = already + mergeAmt
	return mergeAmt
}

func (b *Bot) sellRemainingPositionsIfNeeded(ctx context.Context, market models.Market, orders []models.OrderRecord) {
	if b.positionsSold[market.ConditionID] {
		return
	}
	now := time.Now().Unix()
	if now < (market.EndTS - 60) {
		return
	}

	yesToken, noToken := inferYesNoTokenIDs(market, orders)
	if yesToken == "" || noToken == "" {
		b.positionsSold[market.ConditionID] = true
		return
	}
	yesBal, _ := b.chain.ERC1155BalanceOf(ctx, common.HexToAddress(chain.CTFAddress), mustBigInt(yesToken))
	noBal, _ := b.chain.ERC1155BalanceOf(ctx, common.HexToAddress(chain.CTFAddress), mustBigInt(noToken))
	merged := b.mergedAmounts[market.ConditionID]

	remainingYes := math.Max(0, toFloat6(yesBal)-merged)
	remainingNo := math.Max(0, toFloat6(noBal)-merged)
	if remainingYes <= 0.01 && remainingNo <= 0.01 {
		b.positionsSold[market.ConditionID] = true
		return
	}

	logging.Logger().Printf("Selling remaining positions for %s (YES=%.4f, NO=%.4f)\n", market.MarketSlug, remainingYes, remainingNo)
	yesOutcome, noOutcome := findYesNoOutcomes(market.Outcomes)
	if remainingYes > 0.01 && yesOutcome != nil {
		_ = b.sellPositionMarket(ctx, market, *yesOutcome, remainingYes)
		time.Sleep(500 * time.Millisecond)
	}
	if remainingNo > 0.01 && noOutcome != nil {
		_ = b.sellPositionMarket(ctx, market, *noOutcome, remainingNo)
	}
	b.positionsSold[market.ConditionID] = true
	_ = b.saveOrders()
	_ = b.saveOrderHistory()
}

func (b *Bot) sellPositionMarket(ctx context.Context, market models.Market, outcome models.Outcome, size float64) error {
	// get orderbook bid
	book, err := b.clob.GetOrderBook(ctx, outcome.TokenID)
	if err != nil {
		return err
	}
	bestBid := bestBidFromBook(book)
	if bestBid <= 0 || bestBid < b.cfg.MinSellPrice {
		return fmt.Errorf("best bid %.4f below MIN_SELL_PRICE %.2f", bestBid, b.cfg.MinSellPrice)
	}
	price := bestBid - b.cfg.MarketSellDiscount
	if price < b.cfg.MinSellPrice {
		price = b.cfg.MinSellPrice
	}
	// Round to market tick size (best-effort), to avoid CreateOrder tick validation failures.
	tick := 0.01
	if ts, err := b.clob.GetTickSize(ctx, outcome.TokenID); err == nil {
		if f, ok := parseTickSize(ts); ok && f > 0 {
			tick = f
		}
	}
	price = adjustPriceToTick(price, tick)

	orderArgs := clob.OrderArgs{
		TokenID:    outcome.TokenID,
		Price:      price,
		Size:       size,
		Side:       clob.OrderSideSell,
		FeeRateBps: 0,
		Nonce:      0,
		Expiration: 0,
		Taker:      "",
	}
	signed, _, err := b.clob.CreateOrder(ctx, orderArgs, nil, nil)
	if err != nil {
		return err
	}
	resp, err := b.clob.PostOrder(ctx, signed, clob.OrderTypeGTC)
	if err != nil {
		return err
	}
	orderID := asString(resp["orderID"])
	if orderID == "" {
		orderID = fmt.Sprintf("%d", signed.Salt)
	}
	sizeUSD := price * size
	rev := sizeUSD
	pnl := sizeUSD
	strategy := b.cfg.StrategyName
	rec := models.OrderRecord{
		OrderID:         orderID,
		MarketSlug:      market.MarketSlug,
		ConditionID:     market.ConditionID,
		TokenID:         outcome.TokenID,
		Outcome:         outcome.Outcome,
		Side:            models.OrderSideSell,
		Price:           price,
		Size:            size,
		SizeUSD:         sizeUSD,
		Status:          models.OrderStatusPlaced,
		CreatedAt:       time.Now(),
		Strategy:        &strategy,
		TransactionType: "SELL",
		RevenueUSD:      &rev,
		CostUSD:         floatPtr(0),
		PNLUSD:          &pnl,
	}
	b.orderHistory[rec.OrderID] = rec
	return nil
}

func inferYesNoTokenIDs(market models.Market, orders []models.OrderRecord) (string, string) {
	var yes, no string
	for _, o := range orders {
		u := strings.ToUpper(strings.TrimSpace(o.Outcome))
		if (u == "YES" || u == "UP") && yes == "" {
			yes = o.TokenID
		}
		if (u == "NO" || u == "DOWN") && no == "" {
			no = o.TokenID
		}
	}
	if (yes == "" || no == "") && len(market.Outcomes) > 0 {
		for _, o := range market.Outcomes {
			u := strings.ToUpper(strings.TrimSpace(o.Outcome))
			if (u == "YES" || u == "UP") && yes == "" {
				yes = o.TokenID
			}
			if (u == "NO" || u == "DOWN") && no == "" {
				no = o.TokenID
			}
		}
	}
	return yes, no
}

func mustBigInt(decimal string) *big.Int {
	i := new(big.Int)
	i.SetString(decimal, 10)
	return i
}

func bestBidFromBook(book map[string]any) float64 {
	bids, _ := book["bids"].([]any)
	if len(bids) == 0 {
		return 0
	}
	first, _ := bids[0].(map[string]any)
	if first == nil {
		return 0
	}
	return asFloat(first["price"])
}

func bestAskFromBook(book map[string]any) float64 {
	asks, _ := book["asks"].([]any)
	if len(asks) == 0 {
		return 0
	}
	first, _ := asks[0].(map[string]any)
	if first == nil {
		return 0
	}
	return asFloat(first["price"])
}

func toFloat6(v *big.Int) float64 {
	r := new(big.Rat).SetFrac(v, big.NewInt(1_000_000))
	f, _ := r.Float64()
	return f
}
