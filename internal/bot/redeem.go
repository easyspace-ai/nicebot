package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"limitorderbot/internal/chain"
	"limitorderbot/internal/models"
)

type polymarketPosition struct {
	ConditionID  string  `json:"conditionId"`
	Title        string  `json:"title"`
	Slug         string  `json:"slug"`
	Outcome      string  `json:"outcome"`
	Size         float64 `json:"size"`
	CurPrice     float64 `json:"curPrice"`
	CurrentValue float64 `json:"currentValue"`
	Redeemable   bool    `json:"redeemable"`
}

func (b *Bot) shouldCheckRedemptions(now time.Time) bool {
	if b.lastRedemptionCheck == nil {
		return true
	}
	return now.Sub(*b.lastRedemptionCheck) >= time.Duration(b.cfg.RedeemCheckIntervalSeconds)*time.Second
}

func (b *Bot) checkAndRedeemAll(ctx context.Context) (int, error) {
	// Mirror auto_redeem.py: GET https://data-api.polymarket.com/positions?user=<wallet>
	wallet := b.chain.Address().Hex()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://data-api.polymarket.com/positions?user="+wallet, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("positions api status=%d", resp.StatusCode)
	}
	var positions []polymarketPosition
	if err := json.NewDecoder(resp.Body).Decode(&positions); err != nil {
		return 0, err
	}
	if len(positions) == 0 {
		return 0, nil
	}
	by := map[string][]polymarketPosition{}
	for _, p := range positions {
		if !p.Redeemable {
			continue
		}
		by[p.ConditionID] = append(by[p.ConditionID], p)
	}
	if len(by) == 0 {
		return 0, nil
	}

	success := 0
	for cid, ps := range by {
		condBytes, err := chain.ConditionIDFromHex(cid)
		if err != nil {
			continue
		}
		tx, err := b.chain.RedeemPositions(ctx, condBytes)
		if err != nil {
			continue
		}
		success++

		amount := 0.0
		title := ps[0].Title
		if title == "" {
			title = ps[0].Slug
		}
		for _, p := range ps {
			amount += p.CurrentValue
		}
		// Track redemption in history (best-effort)
		now := time.Now()
		rec := models.OrderRecord{
			OrderID:         fmt.Sprintf("REDEEM-%s-%d", cid[:16], now.Unix()),
			MarketSlug:      title,
			ConditionID:     cid,
			TokenID:         "",
			Outcome:         "REDEEM",
			Side:            models.OrderSideSell,
			Price:           1.0,
			Size:            amount,
			SizeUSD:         amount,
			Status:          models.OrderStatusFilled,
			CreatedAt:       now,
			FilledAt:        &now,
			TransactionType: "REDEEM",
			RevenueUSD:      floatPtr(amount),
			CostUSD:         floatPtr(0),
			PNLUSD:          floatPtr(amount),
		}
		_ = tx // tx hash available for logging (omitted from model for 1:1)
		b.orderHistory[rec.OrderID] = rec
	}

	if success > 0 {
		_ = b.saveOrderHistory()
	}
	return success, nil
}
