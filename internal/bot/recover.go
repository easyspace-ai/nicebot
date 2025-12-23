package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"limitorderbot/internal/logging"
	"limitorderbot/internal/models"
)

func (b *Bot) recoverExistingOrders(ctx context.Context) error {
	// Requires L2; if creds missing GetOrders will fail.
	orders, err := b.clob.GetOrders(ctx, nil)
	if err != nil {
		return nil
	}
	if len(orders) == 0 {
		return nil
	}

	logger := logging.Logger()
	logger.Printf("Recovering %d existing orders from orderbook...\n", len(orders))

	alreadyTracked := func(orderID string) bool {
		for _, group := range b.activeOrders {
			for _, o := range group {
				if o.OrderID == orderID {
					return true
				}
			}
		}
		return false
	}

	recovered := 0
	for _, od := range orders {
		orderID := asString(od["id"])
		conditionID := asString(od["market"])
		tokenID := asString(od["asset_id"])
		sideRaw := strings.ToUpper(asString(od["side"]))
		price := asFloat(od["price"])
		size := asFloat(od["size"])

		if orderID == "" || conditionID == "" {
			continue
		}
		if alreadyTracked(orderID) {
			continue
		}

		marketSlug := fmt.Sprintf("recovered-%s", conditionID[:16])
		outcomeName := "Unknown"

		// Try hydrate via tracked market outcomes
		if m, ok := b.trackedMarkets[conditionID]; ok {
			marketSlug = m.MarketSlug
			for _, o := range m.Outcomes {
				if o.TokenID == tokenID {
					outcomeName = o.Outcome
					break
				}
			}
		} else {
			// Or via previously loaded persisted orders
			for _, group := range b.activeOrders {
				for _, o := range group {
					if o.ConditionID == conditionID && o.TokenID == tokenID {
						marketSlug = o.MarketSlug
						outcomeName = o.Outcome
						break
					}
				}
			}
		}

		side := models.OrderSideBuy
		if sideRaw == "SELL" {
			side = models.OrderSideSell
		}

		rec := models.OrderRecord{
			OrderID:         orderID,
			MarketSlug:      marketSlug,
			ConditionID:     conditionID,
			TokenID:         tokenID,
			Outcome:         outcomeName,
			Side:            side,
			Price:           price,
			Size:            size,
			SizeUSD:         price * size,
			Status:          models.OrderStatusPlaced,
			CreatedAt:       time.Now(),
			TransactionType: string(side),
		}

		// Refresh status to avoid mislabeling
		if det, err := b.clob.GetOrder(ctx, orderID); err == nil {
			status := strings.ToUpper(asString(det["status"]))
			sizeMatched := asFloat(det["size_matched"])
			rec.SizeMatched = &sizeMatched
			if status == "CANCELLED" {
				rec.Status = models.OrderStatusCancelled
			}
		}

		b.activeOrders[conditionID] = append(b.activeOrders[conditionID], rec)
		b.ordersPlaced[conditionID] = true
		b.orderHistory[rec.OrderID] = rec
		recovered++
	}

	if recovered > 0 {
		_ = b.saveOrders()
		_ = b.saveOrderHistory()
	}
	logger.Printf("Recovered %d orders from orderbook\n", recovered)
	return nil
}
