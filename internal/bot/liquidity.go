package bot

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"limitorderbot/internal/clob"
	"limitorderbot/internal/models"
)

// placeLiquidityOrders mirrors python OrderManager.place_liquidity_orders:
// - For each outcome, compute buy at best_bid-spread, sell at best_ask+spread.
// - Size is derived from USD per order: shares = ORDER_SIZE_USD / price.
// - Prices are clamped to [0.01, 0.99] and rounded to 0.01.
// - Best-effort orderbook verification marks orders FAILED if not found.
func (b *Bot) placeLiquidityOrders(ctx context.Context, market models.Market) ([]models.OrderRecord, error) {
	if b.clob == nil {
		return nil, errors.New("clob client not initialized")
	}
	if b.clob.Address() == "" {
		return nil, errors.New("wallet address not available")
	}

	// Balance check (match python): only require USDC for BUY orders.
	bal, _ := b.chain.USDCBalance(ctx)
	required := b.cfg.OrderSizeUSD * 2
	if bal > 0 && bal < required {
		return nil, fmt.Errorf("insufficient balance: $%.2f < $%.2f", bal, required)
	}

	// Ensure we have prices.
	market = b.fillMarketPrices(ctx, []models.Market{market})[0]

	var placed []models.OrderRecord
	for _, outcome := range market.Outcomes {
		if strings.TrimSpace(outcome.TokenID) == "" {
			continue
		}
		if outcome.BestBid == nil || outcome.BestAsk == nil || *outcome.BestBid <= 0 || *outcome.BestAsk <= 0 {
			continue
		}

		buyPrice := adjustPrice(*outcome.BestBid-b.cfg.SpreadOffset, true)
		sellPrice := adjustPrice(*outcome.BestAsk+b.cfg.SpreadOffset, false)

		// BUY
		buyShares := calculateShares(buyPrice, b.cfg.OrderSizeUSD)
		if buyShares > 0 {
			o := b.placeSingleOrderBestEffort(ctx, market, outcome, models.OrderSideBuy, buyPrice, buyShares)
			placed = append(placed, o)
			time.Sleep(500 * time.Millisecond)
		}

		// SELL
		sellShares := calculateShares(sellPrice, b.cfg.OrderSizeUSD)
		if sellShares > 0 {
			o := b.placeSingleOrderBestEffort(ctx, market, outcome, models.OrderSideSell, sellPrice, sellShares)
			placed = append(placed, o)
			time.Sleep(500 * time.Millisecond)
		}
	}

	if len(placed) == 0 {
		return placed, nil
	}
	return b.verifyOrdersInOrderbook(ctx, market, placed), nil
}

func calculateShares(price float64, usd float64) float64 {
	if price <= 0 {
		return 0
	}
	return math.Round((usd/price)*100) / 100
}

func adjustPrice(price float64, _ bool) float64 {
	// Clamp to [0.01, 0.99] and round to 0.01 (match python).
	if price < 0.01 {
		price = 0.01
	}
	if price > 0.99 {
		price = 0.99
	}
	return math.Round(price*100) / 100
}

func (b *Bot) placeSingleOrderBestEffort(
	ctx context.Context,
	market models.Market,
	outcome models.Outcome,
	side models.OrderSide,
	price float64,
	size float64,
) models.OrderRecord {
	now := time.Now()
	sizeUSD := price * size
	strategy := b.cfg.StrategyName

	// Build order args for Go clob client.
	sideStr := clob.OrderSideBuy
	if side == models.OrderSideSell {
		sideStr = clob.OrderSideSell
	}
	args := clob.OrderArgs{
		TokenID:    outcome.TokenID,
		Price:      price,
		Size:       size,
		Side:       sideStr,
		FeeRateBps: 0,
		Nonce:      0,
		Expiration: 0,
		Taker:      "",
	}

	signed, _, err := b.clob.CreateOrder(ctx, args, nil, nil)
	if err != nil {
		msg := err.Error()
		return failedOrderRecord(market, outcome, side, price, size, sizeUSD, &strategy, now, msg)
	}

	resp, err := b.clob.PostOrder(ctx, signed, clob.OrderTypeGTC)
	if err != nil {
		// Mirror python: if the order was signed, it may still have hit the orderbook.
		oid := fmt.Sprintf("%d", signed.Salt)
		msg := fmt.Sprintf("API error (will verify): %v", err)
		rec := orderRecordForSide(market, outcome, side, oid, price, size, sizeUSD, &strategy, now)
		rec.ErrorMessage = &msg
		// Keep status PLACED for verification step.
		return rec
	}

	orderID := asString(resp["orderID"])
	if orderID == "" {
		orderID = fmt.Sprintf("%d", signed.Salt)
	}
	return orderRecordForSide(market, outcome, side, orderID, price, size, sizeUSD, &strategy, now)
}

func orderRecordForSide(
	market models.Market,
	outcome models.Outcome,
	side models.OrderSide,
	orderID string,
	price float64,
	size float64,
	sizeUSD float64,
	strategy *string,
	now time.Time,
) models.OrderRecord {
	rec := models.OrderRecord{
		OrderID:         orderID,
		MarketSlug:      market.MarketSlug,
		ConditionID:     market.ConditionID,
		TokenID:         outcome.TokenID,
		Outcome:         outcome.Outcome,
		Side:            side,
		Price:           price,
		Size:            size,
		SizeUSD:         sizeUSD,
		Status:          models.OrderStatusPlaced,
		CreatedAt:       now,
		Strategy:        strategy,
		TransactionType: string(side),
	}
	if side == models.OrderSideBuy {
		cost := sizeUSD
		pnl := -sizeUSD
		rec.CostUSD = &cost
		rec.RevenueUSD = floatPtr(0)
		rec.PNLUSD = &pnl
		rec.TransactionType = "BUY"
	} else {
		rev := sizeUSD
		pnl := sizeUSD
		rec.RevenueUSD = &rev
		rec.CostUSD = floatPtr(0)
		rec.PNLUSD = &pnl
		rec.TransactionType = "SELL"
	}
	return rec
}

func failedOrderRecord(
	market models.Market,
	outcome models.Outcome,
	side models.OrderSide,
	price float64,
	size float64,
	sizeUSD float64,
	strategy *string,
	now time.Time,
	msg string,
) models.OrderRecord {
	rec := orderRecordForSide(market, outcome, side, "FAILED", price, 0, sizeUSD, strategy, now)
	rec.Status = models.OrderStatusFailed
	rec.ErrorMessage = &msg
	return rec
}

func (b *Bot) verifyOrdersInOrderbook(ctx context.Context, market models.Market, orders []models.OrderRecord) []models.OrderRecord {
	// Match python verify_orders_in_orderbook: pull open orders for the market and mark any missing.
	open, err := b.clob.GetOrders(ctx, &clob.OpenOrderParams{Market: market.ConditionID})
	if err != nil {
		return orders
	}
	active := map[string]struct{}{}
	for _, o := range open {
		id := asString(o["id"])
		if id != "" {
			active[id] = struct{}{}
		}
	}

	var out []models.OrderRecord
	for _, o := range orders {
		if _, ok := active[o.OrderID]; ok {
			o.Status = models.OrderStatusPlaced
			o.ErrorMessage = nil
		} else {
			o.Status = models.OrderStatusFailed
			o.Size = 0
			o.SizeUSD = 0
			if o.ErrorMessage == nil {
				msg := "Order not found in orderbook after placement"
				o.ErrorMessage = &msg
			}
		}
		out = append(out, o)
	}
	return out
}

