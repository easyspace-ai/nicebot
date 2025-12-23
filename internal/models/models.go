package models

import (
	"time"
)

type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

type OrderStatus string

const (
	OrderStatusPending         OrderStatus = "PENDING"
	OrderStatusPlaced          OrderStatus = "PLACED"
	OrderStatusFilled          OrderStatus = "FILLED"
	OrderStatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	OrderStatusCancelled       OrderStatus = "CANCELLED"
	OrderStatusFailed          OrderStatus = "FAILED"
)

type Outcome struct {
	TokenID string   `json:"token_id"`
	Outcome string   `json:"outcome"`
	Price   *float64 `json:"price,omitempty"`
	BestBid *float64 `json:"best_bid,omitempty"`
	BestAsk *float64 `json:"best_ask,omitempty"`
}

type Market struct {
	ConditionID string    `json:"condition_id"`
	MarketSlug  string    `json:"market_slug"`
	Question    string    `json:"question"`
	StartTS     int64     `json:"start_timestamp"`
	EndTS       int64     `json:"end_timestamp"`
	Outcomes    []Outcome `json:"outcomes"`
	IsActive    bool      `json:"is_active"`
	IsResolved  bool      `json:"is_resolved"`
}

func (m Market) StartTime() time.Time { return time.Unix(m.StartTS, 0) }
func (m Market) EndTime() time.Time   { return time.Unix(m.EndTS, 0) }

func (m Market) TimeUntilStart(now time.Time) time.Duration {
	return time.Unix(m.StartTS, 0).Sub(now)
}

type OrderRecord struct {
	OrderID     string      `json:"order_id"`
	MarketSlug  string      `json:"market_slug"`
	ConditionID string      `json:"condition_id"`
	TokenID     string      `json:"token_id"`
	Outcome     string      `json:"outcome"`
	Side        OrderSide   `json:"side"`
	Price       float64     `json:"price"`
	Size        float64     `json:"size"`
	SizeUSD     float64     `json:"size_usd"`
	Status      OrderStatus `json:"status"`

	SizeMatched  *float64   `json:"size_matched,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	FilledAt     *time.Time `json:"filled_at,omitempty"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	Strategy     *string    `json:"strategy,omitempty"`

	TransactionType string   `json:"transaction_type"`
	RevenueUSD      *float64 `json:"revenue_usd,omitempty"`
	CostUSD         *float64 `json:"cost_usd,omitempty"`
	PNLUSD          *float64 `json:"pnl_usd,omitempty"`
}

type BotState struct {
	IsRunning     bool          `json:"is_running"`
	LastCheck     *time.Time    `json:"last_check,omitempty"`
	ActiveMarkets []Market      `json:"active_markets"`
	PendingOrders []OrderRecord `json:"pending_orders"`
	RecentOrders  []OrderRecord `json:"recent_orders"`
	USDCBalance   float64       `json:"usdc_balance"`
	TotalPNL      float64       `json:"total_pnl"`
	ErrorCount    int           `json:"error_count"`
	LastError     *string       `json:"last_error,omitempty"`
}
