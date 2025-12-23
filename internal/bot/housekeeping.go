package bot

import (
	"context"
	"strings"
	"time"

	"limitorderbot/internal/logging"
	"limitorderbot/internal/models"
)

func (b *Bot) cleanupOldMarkets(ctx context.Context, now time.Time) {
	cutoff := now.Add(-24 * time.Hour).Unix()
	var oldCIDs []string
	for cid, m := range b.trackedMarkets {
		if m.EndTS < cutoff {
			oldCIDs = append(oldCIDs, cid)
		}
	}
	if len(oldCIDs) == 0 {
		return
	}
	logging.Logger().Printf("Cleaning up %d old markets and updating order statuses\n", len(oldCIDs))

	statusChanged := false
	for _, cid := range oldCIDs {
		if orders, ok := b.activeOrders[cid]; ok && len(orders) > 0 {
			if b.finalizeOldOrderStatuses(ctx, cid, orders) {
				statusChanged = true
			}
		}

		delete(b.trackedMarkets, cid)
		delete(b.ordersPlaced, cid)
		delete(b.activeOrders, cid)
		delete(b.positionsSold, cid)
		delete(b.lastMergeAttempt, cid)
		delete(b.mergedAmounts, cid)
		delete(b.strategyExecuted, cid)
	}

	_ = b.saveMarkets()
	if statusChanged {
		_ = b.saveOrders()
		_ = b.saveOrderHistory()
	}
}

// finalizeOldOrderStatuses mirrors python _finalize_old_order_statuses:
// if an order is still "open" for a market older than 24h, treat it as cancelled.
func (b *Bot) finalizeOldOrderStatuses(ctx context.Context, conditionID string, orders []models.OrderRecord) bool {
	changed := false
	for i := range orders {
		o := orders[i]
		if o.Status == models.OrderStatusFilled || o.Status == models.OrderStatusCancelled || o.Status == models.OrderStatusFailed {
			continue
		}
		// best-effort refresh
		details, err := b.clob.GetOrder(ctx, o.OrderID)
		if err != nil || details == nil {
			if o.Status == models.OrderStatusPlaced || o.Status == models.OrderStatusPartiallyFilled {
				o.Status = models.OrderStatusCancelled
				changed = true
			}
			orders[i] = o
			b.orderHistory[o.OrderID] = o
			continue
		}
		status := strings.ToUpper(asString(details["status"]))
		if status == "CANCELLED" {
			if o.Status != models.OrderStatusCancelled {
				o.Status = models.OrderStatusCancelled
				changed = true
			}
		} else if o.Status == models.OrderStatusPlaced || o.Status == models.OrderStatusPartiallyFilled {
			// Market is old; if still reported open, mark cancelled to avoid lingering.
			o.Status = models.OrderStatusCancelled
			changed = true
		}
		orders[i] = o
		b.orderHistory[o.OrderID] = o
	}
	b.activeOrders[conditionID] = orders
	return changed
}

// refreshOrphanedOrders mirrors python _refresh_orphaned_orders with a simplified flow:
// - refresh statuses for open orders
// - drop non-live, non-filled orders from active tracking
// - optionally auto-finalize unrecoverable or very old orphan groups
func (b *Bot) refreshOrphanedOrders(ctx context.Context, conditionID string, orders []models.OrderRecord) (bool, []models.OrderRecord) {
	changed := false
	var kept []models.OrderRecord
	for i := range orders {
		o := orders[i]
		if o.Status == models.OrderStatusPlaced || o.Status == models.OrderStatusPartiallyFilled {
			details, err := b.clob.GetOrder(ctx, o.OrderID)
			if err == nil && details != nil {
				status := strings.ToUpper(asString(details["status"]))
				sizeMatched := asFloat(details["size_matched"])
				origSize := asFloat(details["original_size"])
				if origSize == 0 {
					origSize = o.Size
				}
				o.SizeMatched = &sizeMatched
				prev := o.Status
				switch {
				case status == "MATCHED" || (origSize > 0 && sizeMatched >= origSize):
					o.Status = models.OrderStatusFilled
					now := time.Now()
					o.FilledAt = &now
				case sizeMatched > 0:
					o.Status = models.OrderStatusPartiallyFilled
				case status == "CANCELLED":
					o.Status = models.OrderStatusCancelled
				}
				if o.Status != prev {
					changed = true
				}
			} else {
				// If we can't refresh and the orphan market is clearly expired, mark cancelled.
				if b.isOrphanMarketExpired(o.MarketSlug) {
					o.Status = models.OrderStatusCancelled
					changed = true
				}
			}
		}

		b.orderHistory[o.OrderID] = o
		// Keep only potentially-relevant orders.
		if o.Status == models.OrderStatusPlaced || o.Status == models.OrderStatusPartiallyFilled || o.Status == models.OrderStatusFilled {
			kept = append(kept, o)
		} else {
			changed = true
		}
	}

	// If nothing live remains, clear orphaned group.
	if len(kept) == 0 {
		b.clearOrphanGroup(conditionID)
		return true, nil
	}

	// Auto-finalize if missing critical data + wallet empty (python behavior).
	if !b.positionsSold[conditionID] && b.shouldAutoFinalizeOrphan(ctx, conditionID, kept) {
		b.positionsSold[conditionID] = true
		delete(b.activeOrders, conditionID)
		delete(b.lastMergeAttempt, conditionID)
		return true, nil
	}

	return changed, kept
}

func (b *Bot) clearOrphanGroup(conditionID string) {
	delete(b.activeOrders, conditionID)
	delete(b.ordersPlaced, conditionID)
	delete(b.positionsSold, conditionID)
	delete(b.lastMergeAttempt, conditionID)
	delete(b.mergedAmounts, conditionID)
	delete(b.strategyExecuted, conditionID)
}

func (b *Bot) shouldAutoFinalizeOrphan(ctx context.Context, conditionID string, orders []models.OrderRecord) bool {
	// only orphaned groups
	if _, ok := b.trackedMarkets[conditionID]; ok {
		return false
	}
	// live orders? don't finalize
	for _, o := range orders {
		if o.Status == models.OrderStatusPlaced || o.Status == models.OrderStatusPartiallyFilled {
			return false
		}
	}

	// missing data?
	missing := false
	for _, o := range orders {
		if strings.TrimSpace(o.TokenID) == "" || strings.EqualFold(strings.TrimSpace(o.Outcome), "Unknown") {
			missing = true
			break
		}
	}
	if !missing {
		return false
	}

	// If we can verify wallet balances and it's empty, we can finalize.
	cleared, known := b.walletPositionsCleared(ctx, conditionID, orders)
	if known && cleared {
		return true
	}

	// As a fallback: if very old and no live orders, finalize.
	oldest := orders[0].CreatedAt
	for _, o := range orders {
		if o.CreatedAt.Before(oldest) {
			oldest = o.CreatedAt
		}
	}
	return time.Since(oldest) > 24*time.Hour
}

func (b *Bot) isOrphanMarketExpired(marketSlug string) bool {
	// Python: parse btc-updown-15m-{timestamp} and treat end+5m as expired.
	const prefix = "btc-updown-15m-"
	if !strings.Contains(marketSlug, prefix) {
		return false
	}
	parts := strings.Split(marketSlug, prefix)
	if len(parts) < 2 {
		return false
	}
	tsPart := parts[len(parts)-1]
	// token after prefix may include extra suffix; split on '-'
	if strings.Contains(tsPart, "-") {
		tsPart = strings.Split(tsPart, "-")[0]
	}
	start := int64(0)
	for i := 0; i < len(tsPart); i++ {
		c := tsPart[i]
		if c < '0' || c > '9' {
			return false
		}
		start = start*10 + int64(c-'0')
	}
	end := start + 15*60
	return time.Now().Unix() > (end + 300)
}

func (b *Bot) buildOrphanMarket(conditionID string, orders []models.OrderRecord) models.Market {
	now := time.Now().Unix()
	slug := "orphaned-" + conditionID
	if len(orders) > 0 && strings.TrimSpace(orders[0].MarketSlug) != "" {
		slug = orders[0].MarketSlug
	}

	seen := map[string]struct{}{}
	var outs []models.Outcome
	for _, o := range orders {
		tid := strings.TrimSpace(o.TokenID)
		if tid == "" {
			continue
		}
		if _, ok := seen[tid]; ok {
			continue
		}
		seen[tid] = struct{}{}
		name := strings.TrimSpace(o.Outcome)
		if name == "" || strings.EqualFold(name, "Unknown") {
			// best-effort labels
			if len(outs) == 0 {
				name = "Up"
			} else {
				name = "Down"
			}
		}
		outs = append(outs, models.Outcome{TokenID: tid, Outcome: name})
	}

	return models.Market{
		ConditionID: conditionID,
		MarketSlug:  slug,
		Question:    "Orphaned market",
		StartTS:     now - 60,
		EndTS:       now + 3600,
		Outcomes:    outs,
	}
}

