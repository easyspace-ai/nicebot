package bot

import (
	"context"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"limitorderbot/internal/chain"
	"limitorderbot/internal/logging"
	"limitorderbot/internal/models"
)

// hasActiveMarketWork mirrors python bot._has_active_market_work():
// - If any live orders exist, we consider the bot "busy".
// - If any unmerged positions exist (wallet balances), we consider the bot "busy".
func (b *Bot) hasActiveMarketWork(ctx context.Context, now time.Time) (bool, string) {
	// Check 1: live orders
	for cid, orders := range b.activeOrders {
		live := 0
		for _, o := range orders {
			if o.Status == models.OrderStatusPlaced || o.Status == models.OrderStatusPartiallyFilled {
				live++
			}
		}
		if live > 0 {
			name := marketNameForCID(b.trackedMarkets, cid)
			return true, "waiting for " + itoa(live) + " orders to fill in " + name
		}
	}

	// Check 2: unprocessed positions (filled but not merged/sold)
	for cid, orders := range b.activeOrders {
		if b.positionsSold[cid] {
			continue
		}
		hasFilled := false
		for _, o := range orders {
			if o.Status == models.OrderStatusFilled || o.Status == models.OrderStatusPartiallyFilled {
				hasFilled = true
				break
			}
		}
		if !hasFilled {
			continue
		}

		// If clearly expired, don't block new markets (python behavior).
		if m, ok := b.trackedMarkets[cid]; ok {
			if now.Unix() > (m.EndTS + 300) {
				b.positionsSold[cid] = true
				continue
			}
		}

		cleared, known := b.walletPositionsCleared(ctx, cid, orders)
		// If we can't verify, don't block (python behavior).
		if known && !cleared {
			name := marketNameForCID(b.trackedMarkets, cid)
			return true, "waiting to merge positions in " + name
		}
	}

	return false, ""
}

func (b *Bot) walletPositionsCleared(ctx context.Context, conditionID string, orders []models.OrderRecord) (cleared bool, known bool) {
	// Token IDs are the only thing we need; if missing, treat as unknown.
	yesToken, noToken := inferYesNoTokenIDs(models.Market{ConditionID: conditionID}, orders)
	if yesToken == "" || noToken == "" {
		return true, false
	}
	ctf := common.HexToAddress(chain.CTFAddress)
	yesBal, err1 := b.chain.ERC1155BalanceOf(ctx, ctf, mustBigInt(yesToken))
	noBal, err2 := b.chain.ERC1155BalanceOf(ctx, ctf, mustBigInt(noToken))
	if err1 != nil || err2 != nil {
		// If we can't check, don't block to avoid deadlocks.
		return true, false
	}
	// Treat dust as cleared.
	return toFloat6(yesBal) <= 0.01 && toFloat6(noBal) <= 0.01, true
}

func (b *Bot) placeFallbackOrdersIfIdle(ctx context.Context, upcoming []models.Market, now time.Time) {
	if len(upcoming) == 0 {
		return
	}
	hasWork, _ := b.hasActiveMarketWork(ctx, now)
	if hasWork {
		return
	}

	// Pick the nearest future market that is in placement window and not yet placed.
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

	logging.Logger().Printf("Idle state detected. Placing fallback orders for next market: %s\n", pick.MarketSlug)
	orders, err := b.placeSimpleTestOrders(ctx, *pick, 0.49, 10.0)
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

func marketNameForCID(tracked map[string]models.Market, cid string) string {
	if m, ok := tracked[cid]; ok && strings.TrimSpace(m.MarketSlug) != "" {
		return m.MarketSlug
	}
	if len(cid) > 16 {
		return cid[:16]
	}
	return cid
}

func itoa(n int) string {
	// small helper without pulling strconv in multiple files
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	digits := []byte{}
	for n > 0 {
		d := byte(n % 10)
		digits = append([]byte{'0' + d}, digits...)
		n /= 10
	}
	return string(digits)
}

