package bot

import (
	"context"
	"time"

	"limitorderbot/internal/logging"
	"limitorderbot/internal/models"
)

func (b *Bot) placeFallbackLiquidityIfIdle(ctx context.Context, upcoming []models.Market, now time.Time) {
	if len(upcoming) == 0 {
		return
	}
	hasWork, _ := b.hasActiveMarketWork(ctx, now)
	if hasWork {
		return
	}

	var pick *models.Market
	for i := range upcoming {
		m := upcoming[i]
		if m.StartTS <= now.Unix() {
			continue
		}
		if b.ordersPlaced[m.ConditionID] {
			continue
		}
		if !shouldPlaceOrders(b.cfg, m, now) {
			continue
		}
		if pick == nil || m.StartTS < pick.StartTS {
			tmp := m
			pick = &tmp
		}
	}
	if pick == nil {
		return
	}

	logging.Logger().Printf("Idle state detected. Placing fallback liquidity orders for next market: %s\n", pick.MarketSlug)
	orders, err := b.placeLiquidityOrders(ctx, *pick)
	if err != nil {
		b.recordError(err)
		return
	}
	if len(orders) == 0 {
		return
	}
	b.ordersPlaced[pick.ConditionID] = true
	b.activeOrders[pick.ConditionID] = orders
	for _, o := range orders {
		b.orderHistory[o.OrderID] = o
	}
	_ = b.saveOrders()
	_ = b.saveOrderHistory()
}

