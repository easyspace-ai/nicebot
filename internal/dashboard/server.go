package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"limitorderbot/internal/bot"
	"limitorderbot/internal/config"
	"limitorderbot/internal/models"
)

type Server struct {
	cfg config.Config
	bot *bot.Bot
	tpl *template.Template
}

func New(cfg config.Config, b *bot.Bot) (*Server, error) {
	// Use the existing HTML as-is for 1:1 UI.
	tplPath := filepath.Join("templates", "dashboard.html")
	tpl, err := template.ParseFiles(tplPath)
	if err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, bot: b, tpl: tpl}, nil
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/markets", s.handleMarkets)
	mux.HandleFunc("/api/orders", s.handleOrders)
	mux.HandleFunc("/api/market-history", s.handleMarketHistory)
	mux.HandleFunc("/api/statistics", s.handleStatistics)
	mux.HandleFunc("/api/strategy-statistics", s.handleStrategyStatistics)
	mux.HandleFunc("/api/logs", s.handleLogs)

	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", s.cfg.DashboardHost, s.cfg.DashboardPort),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	return srv.ListenAndServe()
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	_ = s.tpl.Execute(w, map[string]any{})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	state := s.bot.GetState()
	now := time.Now()
	last := now
	if state.LastCheck != nil {
		last = *state.LastCheck
	}
	next := last.Add(time.Duration(s.cfg.CheckIntervalSeconds) * time.Second)
	minBalanceNeeded := s.cfg.OrderSizeUSD * 2
	hasSufficient := state.USDCBalance >= minBalanceNeeded

	resp := map[string]any{
		"is_running":             state.IsRunning,
		"last_check":             last.Format(time.RFC3339Nano),
		"next_check":             next.Format(time.RFC3339Nano),
		"check_interval_seconds": s.cfg.CheckIntervalSeconds,
		"usdc_balance":           round2(state.USDCBalance),
		"total_pnl":              round2(state.TotalPNL),
		"error_count":            state.ErrorCount,
		"last_error":             state.LastError,
		"active_markets_count":   len(state.ActiveMarkets),
		"pending_orders_count":   len(state.PendingOrders),
		"wallet_address":         s.botAddress(),
		"balance_warning":        !hasSufficient,
		"balance_error_count":    0,
		"min_balance_needed":     minBalanceNeeded,
	}
	writeJSON(w, resp)
}

func (s *Server) botAddress() string {
	return s.bot.WalletAddress()
}

func (s *Server) handleMarkets(w http.ResponseWriter, r *http.Request) {
	state := s.bot.GetState()
	now := time.Now()

	var markets []map[string]any
	for _, m := range state.ActiveMarkets {
		startIso := m.StartTime().Format(time.RFC3339Nano)
		endIso := m.EndTime().Format(time.RFC3339Nano)
		sec := m.TimeUntilStart(now).Seconds()
		markets = append(markets, map[string]any{
			"market_slug":                m.MarketSlug,
			"question":                   m.Question,
			"start_timestamp":            m.StartTS,
			"start_datetime":             startIso,
			"end_datetime":               endIso,
			"time_until_start":           int64(sec),
			"time_until_start_formatted": formatTimeDelta(sec),
			"is_active":                  m.IsActive,
			"is_resolved":                m.IsResolved,
			"outcomes":                   outcomesForAPI(m.Outcomes),
			"orders_placed":              false,
		})
	}
	sort.Slice(markets, func(i, j int) bool {
		return markets[i]["start_timestamp"].(int64) < markets[j]["start_timestamp"].(int64)
	})
	if len(markets) > 10 {
		markets = markets[:10]
	}
	writeJSON(w, map[string]any{"markets": markets})
}

func outcomesForAPI(outs []models.Outcome) []map[string]any {
	var res []map[string]any
	for _, o := range outs {
		var p, bb, ba any
		if o.Price != nil {
			p = round3(*o.Price)
		}
		if o.BestBid != nil {
			bb = round3(*o.BestBid)
		}
		if o.BestAsk != nil {
			ba = round3(*o.BestAsk)
		}
		res = append(res, map[string]any{
			"outcome":  o.Outcome,
			"price":    p,
			"best_bid": bb,
			"best_ask": ba,
		})
	}
	return res
}

func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	state := s.bot.GetState()
	var pending []map[string]any
	for _, o := range state.PendingOrders {
		pending = append(pending, map[string]any{
			"order_id":    shorten(o.OrderID),
			"market_slug": o.MarketSlug,
			"outcome":     o.Outcome,
			"side":        string(o.Side),
			"price":       round3(o.Price),
			"size":        round2(o.Size),
			"size_usd":    round2(o.SizeUSD),
			"status":      string(o.Status),
			"strategy":    o.Strategy,
			"created_at":  o.CreatedAt.Format(time.RFC3339Nano),
			"filled_at":   timeOrNil(o.FilledAt),
		})
	}
	var recent []map[string]any
	for _, o := range state.RecentOrders {
		recent = append(recent, map[string]any{
			"order_id":      shorten(o.OrderID),
			"market_slug":   o.MarketSlug,
			"outcome":       o.Outcome,
			"side":          string(o.Side),
			"price":         round3(o.Price),
			"size":          round2(o.Size),
			"size_usd":      round2(o.SizeUSD),
			"status":        string(o.Status),
			"strategy":      o.Strategy,
			"created_at":    o.CreatedAt.Format(time.RFC3339Nano),
			"filled_at":     timeOrNil(o.FilledAt),
			"error_message": o.ErrorMessage,
		})
		if len(recent) >= 100 {
			break
		}
	}
	writeJSON(w, map[string]any{"pending_orders": pending, "recent_orders": recent})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	path := s.cfg.LogFile
	b, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, map[string]any{"logs": []string{}})
		return
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) > 50 {
		lines = lines[len(lines)-50:]
	}
	writeJSON(w, map[string]any{"logs": lines})
}

func (s *Server) handleMarketHistory(w http.ResponseWriter, r *http.Request) {
	orders, _ := loadHistoryFile("order_history.json")
	type agg struct {
		marketSlug string
		strategy   string
		createdAt  time.Time
		totalCost  float64
		totalRev   float64
		filled     int
		total      int
		open       bool
	}
	by := map[string]*agg{}
	for _, o := range orders {
		a := by[o.ConditionID]
		if a == nil {
			a = &agg{marketSlug: o.MarketSlug, strategy: deref(o.Strategy, "None"), createdAt: o.CreatedAt}
			by[o.ConditionID] = a
		}
		if o.CreatedAt.Before(a.createdAt) {
			a.createdAt = o.CreatedAt
		}
		if o.TransactionType == "BUY" {
			a.total++
			if o.Status == models.OrderStatusFilled || o.Status == models.OrderStatusPartiallyFilled {
				a.filled++
			}
		}
		if o.Status == models.OrderStatusPlaced || o.Status == models.OrderStatusPartiallyFilled {
			a.open = true
		}
		if (o.Status == models.OrderStatusFilled || o.Status == models.OrderStatusPartiallyFilled) && o.Side == models.OrderSideBuy {
			if o.CostUSD != nil {
				a.totalCost += *o.CostUSD
			} else {
				a.totalCost += o.SizeUSD
			}
		}
		if (o.Status == models.OrderStatusFilled || o.Status == models.OrderStatusPartiallyFilled) && o.Side == models.OrderSideSell {
			if o.RevenueUSD != nil {
				a.totalRev += *o.RevenueUSD
			} else {
				a.totalRev += o.SizeUSD
			}
		}
	}

	type row struct {
		MarketSlug   string  `json:"market_slug"`
		ConditionID  string  `json:"condition_id"`
		Strategy     string  `json:"strategy"`
		Status       string  `json:"status"`
		Result       string  `json:"result"`
		TotalCost    float64 `json:"total_cost"`
		TotalRevenue float64 `json:"total_revenue"`
		PNL          float64 `json:"pnl"`
		FilledCount  int     `json:"filled_count"`
		TotalCount   int     `json:"total_count"`
		CreatedAt    string  `json:"created_at"`
	}
	var rows []row
	for cid, a := range by {
		status := fmt.Sprintf("FILLED %d/%d", a.filled, a.total)
		result := "N/A"
		if a.open {
			result = "OPEN"
		} else if a.totalRev > 0 {
			if a.totalRev >= a.totalCost {
				result = "SUCCESS"
			} else {
				result = "FAILED"
			}
		}
		pnl := a.totalRev - a.totalCost
		if a.open {
			a.totalCost = 0
			a.totalRev = 0
			pnl = 0
		}
		rows = append(rows, row{
			MarketSlug:   a.marketSlug,
			ConditionID:  cid,
			Strategy:     a.strategy,
			Status:       status,
			Result:       result,
			TotalCost:    round2(a.totalCost),
			TotalRevenue: round2(a.totalRev),
			PNL:          round2(pnl),
			FilledCount:  a.filled,
			TotalCount:   a.total,
			CreatedAt:    a.createdAt.Format(time.RFC3339Nano),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt > rows[j].CreatedAt })
	if len(rows) > 100 {
		rows = rows[:100]
	}
	writeJSON(w, map[string]any{"markets": rows})
}

func (s *Server) handleStatistics(w http.ResponseWriter, r *http.Request) {
	orders, _ := loadHistoryFile("order_history.json")
	by := map[string][]models.OrderRecord{}
	var pnl float64
	for _, o := range orders {
		by[o.ConditionID] = append(by[o.ConditionID], o)
		if o.PNLUSD != nil {
			pnl += *o.PNLUSD
		}
	}
	totalMarkets := len(by)
	success := 0
	fail := 0
	for _, ords := range by {
		var yes, no float64
		for _, o := range ords {
			if o.Status != models.OrderStatusFilled && o.Status != models.OrderStatusPartiallyFilled {
				continue
			}
			u := strings.ToUpper(strings.TrimSpace(o.Outcome))
			if u == "YES" || u == "UP" {
				yes += o.Size
			}
			if u == "NO" || u == "DOWN" {
				no += o.Size
			}
		}
		if yes > 0 && no > 0 {
			success++
		} else {
			fail++
		}
	}
	writeJSON(w, map[string]any{
		"total_markets":       totalMarkets,
		"successful_trades":   success,
		"unsuccessful_trades": fail,
		"total_pnl":           round2(pnl),
	})
}

func (s *Server) handleStrategyStatistics(w http.ResponseWriter, r *http.Request) {
	orders, _ := loadHistoryFile("order_history.json")
	byStrat := map[string][]models.OrderRecord{}
	for _, o := range orders {
		byStrat[deref(o.Strategy, "None")] = append(byStrat[deref(o.Strategy, "None")], o)
	}
	type row struct {
		StrategyName       string  `json:"strategy_name"`
		TotalMarkets       int     `json:"total_markets"`
		SuccessfulTrades   int     `json:"successful_trades"`
		UnsuccessfulTrades int     `json:"unsuccessful_trades"`
		TotalPNL           float64 `json:"total_pnl"`
	}
	var rows []row
	for name, ords := range byStrat {
		byMarket := map[string][]models.OrderRecord{}
		var pnl float64
		for _, o := range ords {
			byMarket[o.ConditionID] = append(byMarket[o.ConditionID], o)
			if o.PNLUSD != nil {
				pnl += *o.PNLUSD
			}
		}
		success := 0
		fail := 0
		for _, mos := range byMarket {
			var yes, no float64
			for _, o := range mos {
				if o.Status != models.OrderStatusFilled && o.Status != models.OrderStatusPartiallyFilled {
					continue
				}
				u := strings.ToUpper(strings.TrimSpace(o.Outcome))
				if u == "YES" || u == "UP" {
					yes += o.Size
				}
				if u == "NO" || u == "DOWN" {
					no += o.Size
				}
			}
			if yes > 0 && no > 0 {
				success++
			} else {
				fail++
			}
		}
		rows = append(rows, row{
			StrategyName:       name,
			TotalMarkets:       len(byMarket),
			SuccessfulTrades:   success,
			UnsuccessfulTrades: fail,
			TotalPNL:           round2(pnl),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].StrategyName < rows[j].StrategyName })
	writeJSON(w, map[string]any{"strategies": rows})
}

func loadHistoryFile(path string) ([]models.OrderRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var arr []map[string]any
	if err := json.Unmarshal(b, &arr); err != nil {
		return nil, err
	}
	var out []models.OrderRecord
	for _, m := range arr {
		o, err := parseHistoryOrder(m)
		if err == nil {
			out = append(out, o)
		}
	}
	return out, nil
}

func parseHistoryOrder(m map[string]any) (models.OrderRecord, error) {
	// Minimal parsing: fields we use for stats.
	var created time.Time
	if s, ok := m["created_at"].(string); ok {
		created, _ = time.Parse(time.RFC3339Nano, s)
		if created.IsZero() {
			created, _ = time.Parse(time.RFC3339, s)
		}
	}
	return models.OrderRecord{
		OrderID:         asStr(m["order_id"]),
		MarketSlug:      asStr(m["market_slug"]),
		ConditionID:     asStr(m["condition_id"]),
		Outcome:         asStr(m["outcome"]),
		Side:            models.OrderSide(asStr(m["side"])),
		Price:           asF(m["price"]),
		Size:            asF(m["size"]),
		SizeUSD:         asF(m["size_usd"]),
		Status:          models.OrderStatus(asStr(m["status"])),
		CreatedAt:       created,
		TransactionType: asStr(m["transaction_type"]),
		Strategy:        strPtrOrNil(m["strategy"]),
		PNLUSD:          floatPtrOrNil(m["pnl_usd"]),
		CostUSD:         floatPtrOrNil(m["cost_usd"]),
		RevenueUSD:      floatPtrOrNil(m["revenue_usd"]),
	}, nil
}

func asStr(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func asF(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		var f float64
		_, _ = fmt.Sscanf(t, "%f", &f)
		return f
	default:
		return 0
	}
}

func floatPtrOrNil(v any) *float64 {
	if v == nil {
		return nil
	}
	f := asF(v)
	return &f
}

func strPtrOrNil(v any) *string {
	if v == nil {
		return nil
	}
	s := asStr(v)
	if s == "" || s == "<nil>" {
		return nil
	}
	return &s
}

func timeOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339Nano)
}

func deref(s *string, def string) string {
	if s == nil || *s == "" {
		return def
	}
	return *s
}

func shorten(id string) string {
	if len(id) <= 16 {
		return id
	}
	return id[:16] + "..."
}

func formatTimeDelta(seconds float64) string {
	if seconds < 0 {
		return "Started"
	}
	if seconds < 60 {
		return fmt.Sprintf("%ds", int(seconds))
	}
	if seconds < 3600 {
		min := int(seconds / 60)
		sec := int(math.Mod(seconds, 60))
		return fmt.Sprintf("%dm %ds", min, sec)
	}
	h := int(seconds / 3600)
	m := int(math.Mod(seconds, 3600) / 60)
	return fmt.Sprintf("%dh %dm", h, m)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	_ = enc.Encode(v)
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }
func round3(x float64) float64 { return math.Round(x*1000) / 1000 }
