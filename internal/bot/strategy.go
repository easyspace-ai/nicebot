package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"limitorderbot/internal/chain"
	"limitorderbot/internal/logging"
	"limitorderbot/internal/models"
)

func (b *Bot) checkStrategyExecution(ctx context.Context, now time.Time) {
	strat, ok := b.cfg.Strategy()
	if !ok || !strat.Enabled {
		return
	}

	for cid, orders := range b.activeOrders {
		if b.strategyExecuted[cid] {
			continue
		}
		market, ok := b.trackedMarkets[cid]
		if !ok {
			continue
		}
		// Only apply to orders that belong to current strategy (or legacy nil)
		if len(orders) == 0 {
			continue
		}
		// Find strategy tag (if present)
		strategyName := b.cfg.StrategyName
		if orders[0].Strategy != nil && *orders[0].Strategy != "" {
			strategyName = *orders[0].Strategy
		}
		if strings.TrimSpace(strategyName) != b.cfg.StrategyName {
			continue
		}

		// Wait until market started
		if now.Unix() < market.StartTS {
			continue
		}
		sinceStart := now.Sub(market.StartTime())
		if sinceStart < time.Duration(strat.ExitTimeoutSeconds)*time.Second {
			continue
		}

		logging.Logger().Printf("Strategy '%s' timeout reached for %s (sinceStart=%ds, timeout=%ds)\n",
			b.cfg.StrategyName, market.MarketSlug, int(sinceStart.Seconds()), strat.ExitTimeoutSeconds)

		// Step 1: cancel unfilled
		if strat.CancelUnfilled {
			for i := range orders {
				if orders[i].Status == models.OrderStatusPlaced || orders[i].Status == models.OrderStatusPartiallyFilled {
					_, _ = b.clob.Cancel(ctx, orders[i].OrderID)
					orders[i].Status = models.OrderStatusCancelled
					b.orderHistory[orders[i].OrderID] = orders[i]
				}
			}
		}

		// Step 2: merge, then sell leftovers immediately (not waiting for market end)
		if strat.MarketSellFilled {
			merged := b.mergePositionsIfPossible(ctx, market, orders)
			if merged > 0 {
				b.trackMerge(market, merged)
			}
			// Force sell leftovers now
			b.sellLeftoversNow(ctx, market, orders)
		}

		b.activeOrders[cid] = orders
		b.strategyExecuted[cid] = true
		_ = b.saveOrders()
		_ = b.saveOrderHistory()
	}
}

func (b *Bot) sellLeftoversNow(ctx context.Context, market models.Market, orders []models.OrderRecord) {
	yesToken, noToken := inferYesNoTokenIDs(market, orders)
	if yesToken == "" || noToken == "" {
		return
	}
	ctf := common.HexToAddress(chain.CTFAddress)
	yesBal, _ := b.chain.ERC1155BalanceOf(ctx, ctf, mustBigInt(yesToken))
	noBal, _ := b.chain.ERC1155BalanceOf(ctx, ctf, mustBigInt(noToken))
	_ = yesBal
	_ = noBal
	// Reuse existing sell logic but bypass end-time check by calling sellPositionMarket directly.
	yesOutcome, noOutcome := findYesNoOutcomes(market.Outcomes)
	merged := b.mergedAmounts[market.ConditionID]
	remainingYes := toFloat6(yesBal) - merged
	remainingNo := toFloat6(noBal) - merged
	if yesOutcome != nil && remainingYes > 0.01 {
		_ = b.sellPositionMarket(ctx, market, *yesOutcome, remainingYes)
		time.Sleep(500 * time.Millisecond)
	}
	if noOutcome != nil && remainingNo > 0.01 {
		_ = b.sellPositionMarket(ctx, market, *noOutcome, remainingNo)
	}
	b.positionsSold[market.ConditionID] = true
}

func (b *Bot) trackMerge(market models.Market, merged float64) {
	now := time.Now()
	rev := merged
	rec := models.OrderRecord{
		OrderID:         fmt.Sprintf("MERGE-%s-%d", market.ConditionID[:16], now.Unix()),
		MarketSlug:      market.MarketSlug,
		ConditionID:     market.ConditionID,
		TokenID:         "",
		Outcome:         "MERGE",
		Side:            models.OrderSideSell,
		Price:           1.0,
		Size:            merged,
		SizeUSD:         merged,
		Status:          models.OrderStatusFilled,
		CreatedAt:       now,
		FilledAt:        &now,
		TransactionType: "MERGE",
		RevenueUSD:      &rev,
		CostUSD:         floatPtr(0),
		PNLUSD:          &rev,
	}
	b.orderHistory[rec.OrderID] = rec
}
