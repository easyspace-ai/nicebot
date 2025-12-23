package bot

import (
	"encoding/json"
	"os"
	"sort"
	"time"

	"limitorderbot/internal/models"
)

func (b *Bot) saveMarkets() error {
	out := map[string]any{}
	for cid, m := range b.trackedMarkets {
		outs := make([]any, 0, len(m.Outcomes))
		for _, o := range m.Outcomes {
			outs = append(outs, map[string]any{
				"token_id": o.TokenID,
				"outcome":  o.Outcome,
			})
		}
		out[cid] = map[string]any{
			"condition_id":    m.ConditionID,
			"market_slug":     m.MarketSlug,
			"question":        m.Question,
			"start_timestamp": m.StartTS,
			"end_timestamp":   m.EndTS,
			"is_active":       m.IsActive,
			"is_resolved":     m.IsResolved,
			"outcomes":        outs,
		}
	}
	bts, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(b.marketsFile, bts, 0o644)
}

func (b *Bot) loadMarkets() error {
	raw, err := os.ReadFile(b.marketsFile)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	for cid, v := range m {
		obj, _ := v.(map[string]any)
		if obj == nil {
			continue
		}
		outcomes := []models.Outcome{}
		if arr, ok := obj["outcomes"].([]any); ok {
			for _, ov := range arr {
				om, _ := ov.(map[string]any)
				if om == nil {
					continue
				}
				outcomes = append(outcomes, models.Outcome{
					TokenID: asString(om["token_id"]),
					Outcome: asString(om["outcome"]),
				})
			}
		}
		b.trackedMarkets[cid] = models.Market{
			ConditionID: asString(obj["condition_id"]),
			MarketSlug:  asString(obj["market_slug"]),
			Question:    asString(obj["question"]),
			StartTS:     int64(asFloat(obj["start_timestamp"])),
			EndTS:       int64(asFloat(obj["end_timestamp"])),
			Outcomes:    outcomes,
			IsActive:    asBool(obj["is_active"]),
			IsResolved:  asBool(obj["is_resolved"]),
		}
	}
	return nil
}

func (b *Bot) saveOrders() error {
	out := map[string]any{}
	for cid, orders := range b.activeOrders {
		arr := make([]any, 0, len(orders))
		for _, o := range orders {
			arr = append(arr, serializeOrder(o))
		}
		out[cid] = arr
	}
	bts, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(b.ordersFile, bts, 0o644)
}

func (b *Bot) loadOrders() error {
	raw, err := os.ReadFile(b.ordersFile)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	for cid, v := range m {
		arr, _ := v.([]any)
		if arr == nil {
			continue
		}
		var orders []models.OrderRecord
		hasOpen := false
		for _, ov := range arr {
			om, _ := ov.(map[string]any)
			if om == nil {
				continue
			}
			o, err := parseOrderRecord(om)
			if err != nil {
				continue
			}
			if o.Status == models.OrderStatusPlaced || o.Status == models.OrderStatusPartiallyFilled {
				hasOpen = true
			}
			orders = append(orders, o)
			b.orderHistory[o.OrderID] = o
		}
		if len(orders) > 0 {
			b.activeOrders[cid] = orders
		}
		if hasOpen {
			b.ordersPlaced[cid] = true
		}
	}
	return nil
}

func (b *Bot) saveOrderHistory() error {
	hist := make([]models.OrderRecord, 0, len(b.orderHistory))
	for _, o := range b.orderHistory {
		hist = append(hist, o)
	}
	sort.Slice(hist, func(i, j int) bool { return hist[i].CreatedAt.After(hist[j].CreatedAt) })
	arr := make([]any, 0, len(hist))
	for _, o := range hist {
		arr = append(arr, serializeOrder(o))
	}
	bts, err := json.MarshalIndent(arr, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(b.orderHistoryFile, bts, 0o644)
}

func (b *Bot) loadOrderHistory() error {
	raw, err := os.ReadFile(b.orderHistoryFile)
	if err != nil {
		return nil
	}
	var arr []any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return err
	}
	for _, v := range arr {
		om, _ := v.(map[string]any)
		if om == nil {
			continue
		}
		o, err := parseOrderRecord(om)
		if err != nil {
			continue
		}
		b.orderHistory[o.OrderID] = o
	}
	return nil
}

func serializeOrder(o models.OrderRecord) map[string]any {
	var filledAt any
	if o.FilledAt != nil {
		filledAt = o.FilledAt.Format(time.RFC3339Nano)
	} else {
		filledAt = nil
	}
	var sizeMatched any
	if o.SizeMatched != nil {
		sizeMatched = *o.SizeMatched
	}
	return map[string]any{
		"order_id":         o.OrderID,
		"market_slug":      o.MarketSlug,
		"condition_id":     o.ConditionID,
		"token_id":         o.TokenID,
		"outcome":          o.Outcome,
		"side":             string(o.Side),
		"price":            o.Price,
		"size":             o.Size,
		"size_usd":         o.SizeUSD,
		"status":           string(o.Status),
		"size_matched":     sizeMatched,
		"created_at":       o.CreatedAt.Format(time.RFC3339Nano),
		"filled_at":        filledAt,
		"error_message":    o.ErrorMessage,
		"strategy":         o.Strategy,
		"transaction_type": o.TransactionType,
		"revenue_usd":      o.RevenueUSD,
		"cost_usd":         o.CostUSD,
		"pnl_usd":          o.PNLUSD,
	}
}

func parseOrderRecord(m map[string]any) (models.OrderRecord, error) {
	createdAt, _ := time.Parse(time.RFC3339Nano, asString(m["created_at"]))
	if createdAt.IsZero() {
		// try RFC3339
		createdAt, _ = time.Parse(time.RFC3339, asString(m["created_at"]))
	}

	var filledAt *time.Time
	if s := asString(m["filled_at"]); s != "" && s != "<nil>" {
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			t, _ = time.Parse(time.RFC3339, s)
		}
		if !t.IsZero() {
			filledAt = &t
		}
	}

	var sizeMatched *float64
	if v, ok := m["size_matched"]; ok && v != nil {
		f := asFloat(v)
		sizeMatched = &f
	}

	var errMsg *string
	if v := m["error_message"]; v != nil {
		s := asString(v)
		if s != "" && s != "<nil>" {
			errMsg = &s
		}
	}

	var strategy *string
	if v := m["strategy"]; v != nil {
		s := asString(v)
		if s != "" && s != "<nil>" {
			strategy = &s
		}
	}

	rec := models.OrderRecord{
		OrderID:         asString(m["order_id"]),
		MarketSlug:      asString(m["market_slug"]),
		ConditionID:     asString(m["condition_id"]),
		TokenID:         asString(m["token_id"]),
		Outcome:         asString(m["outcome"]),
		Side:            models.OrderSide(asString(m["side"])),
		Price:           asFloat(m["price"]),
		Size:            asFloat(m["size"]),
		SizeUSD:         asFloat(m["size_usd"]),
		Status:          models.OrderStatus(asString(m["status"])),
		SizeMatched:     sizeMatched,
		CreatedAt:       createdAt,
		FilledAt:        filledAt,
		ErrorMessage:    errMsg,
		Strategy:        strategy,
		TransactionType: asString(m["transaction_type"]),
	}
	return rec, nil
}
