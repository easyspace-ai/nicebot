package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sort"
	"sync"
	"time"

	"limitorderbot/internal/chain"
	"limitorderbot/internal/clob"
	"limitorderbot/internal/config"
	"limitorderbot/internal/gamma"
	"limitorderbot/internal/logging"
	"limitorderbot/internal/models"
)

type Bot struct {
	cfg      config.Config
	discover *gamma.Discovery
	clob     *clob.Client
	chain    *chain.Client

	mu sync.Mutex

	state models.BotState

	trackedMarkets map[string]models.Market
	ordersPlaced   map[string]bool
	activeOrders   map[string][]models.OrderRecord
	orderHistory   map[string]models.OrderRecord

	lastMergeAttempt map[string]time.Time
	mergedAmounts    map[string]float64
	positionsSold    map[string]bool
	strategyExecuted map[string]bool

	lastRedemptionCheck *time.Time

	ordersFile       string
	orderHistoryFile string
	marketsFile      string
}

func New(cfg config.Config) (*Bot, error) {
	closeFn, err := logging.Configure(cfg.LogLevel, cfg.LogFile)
	if err != nil {
		return nil, err
	}
	_ = closeFn // log file close is process-scoped in this port

	cc, err := clob.NewClient(cfg.ClobAPIURL, cfg.ChainID, cfg.PrivateKey, cfg.SignatureType, cfg.FunderAddress)
	if err != nil {
		return nil, err
	}
	ch, err := chain.New(cfg.RPCURL, cfg.PrivateKey, cfg.ChainID)
	if err != nil {
		return nil, err
	}

	b := &Bot{
		cfg:              cfg,
		discover:         gamma.New(cfg.GammaAPIBaseURL),
		clob:             cc,
		chain:            ch,
		trackedMarkets:   map[string]models.Market{},
		ordersPlaced:     map[string]bool{},
		activeOrders:     map[string][]models.OrderRecord{},
		orderHistory:     map[string]models.OrderRecord{},
		lastMergeAttempt: map[string]time.Time{},
		mergedAmounts:    map[string]float64{},
		positionsSold:    map[string]bool{},
		strategyExecuted: map[string]bool{},
		ordersFile:       "bot_orders.json",
		orderHistoryFile: "order_history.json",
		marketsFile:      "markets_state.json",
	}

	// initial state
	b.state.ActiveMarkets = []models.Market{}
	b.state.PendingOrders = []models.OrderRecord{}
	b.state.RecentOrders = []models.OrderRecord{}
	return b, nil
}

func (b *Bot) Close() error {
	return b.chain.Close()
}

func (b *Bot) Start(ctx context.Context) error {
	logger := logging.Logger()
	logger.Println(strings.Repeat("=", 60))
	logger.Println("Starting Polymarket Limit Order Bot (Go)")
	logger.Println(strings.Repeat("=", 60))
	logger.Printf("Wallet address: %s\n", b.clob.Address())
	logger.Printf("Order size: $%.2f per order\n", b.cfg.OrderSizeUSD)
	logger.Printf("Spread offset: %.4f\n", b.cfg.SpreadOffset)
	logger.Printf("Order placement window: %d-%d min before start\n", b.cfg.OrderPlacementMinMinutes, b.cfg.OrderPlacementMaxMinutes)
	logger.Println(strings.Repeat("=", 60))

	// Load persisted state
	_ = b.loadMarkets()
	_ = b.loadOrderHistory()
	_ = b.loadOrders()

	// Initialize balance immediately
	bal, err := b.chain.USDCBalance(ctx)
	if err != nil {
		bal = 0
	}

	// Derive creds best-effort
	creds, err := b.clob.CreateOrDeriveAPICreds(ctx, 0)
	if err == nil && creds.APIKey != "" {
		b.clob.SetCreds(creds)
		logger.Println("CLOB API creds derived and set successfully")
		// Mirror python: try to update L2 balance allowance on startup.
		b.updateL2BalanceAllowanceBestEffort(ctx)
	} else {
		logger.Printf("WARNING: Could not derive API creds (read-only mode): %v\n", err)
	}

	// Recover existing open orders from orderbook (if L2 auth available)
	if b.clob != nil {
		_ = b.recoverExistingOrders(ctx)
	}

	now := time.Now()
	b.mu.Lock()
	b.state.IsRunning = true
	b.state.USDCBalance = bal
	b.state.LastCheck = &now
	b.mu.Unlock()
	return nil
}

func (b *Bot) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state.IsRunning = false
}

func (b *Bot) GetState() models.BotState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

func (b *Bot) WalletAddress() string {
	if b.clob == nil {
		return ""
	}
	return b.clob.Address()
}

func (b *Bot) OrdersPlaced(conditionID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ordersPlaced[conditionID]
}

func (b *Bot) RunOnce(ctx context.Context) {
	now := time.Now()
	b.mu.Lock()
	b.state.LastCheck = &now
	b.mu.Unlock()

	logger := logging.Logger()

	// Step 0: auto redeem (periodic)
	if b.shouldCheckRedemptions(now) {
		if redeemed, err := b.checkAndRedeemAll(ctx); err != nil {
			logger.Printf("Redemption check error: %v\n", err)
		} else if redeemed > 0 {
			logger.Printf("✓ Claimed winnings from %d resolved markets\n", redeemed)
		}
		t := now
		b.lastRedemptionCheck = &t
	}

	// Step 1: discover markets
	logger.Println("Discovering BTC 15-minute markets...")
	markets, err := b.discover.DiscoverBTC15mMarkets(ctx)
	if err != nil {
		b.recordError(err)
		return
	}
	upcoming := b.filterUpcoming(markets, now)
	// Fill market prices for dashboard (best-effort)
	upcoming = b.fillMarketPrices(ctx, upcoming)

	b.mu.Lock()
	b.state.ActiveMarkets = upcoming
	b.mu.Unlock()
	logger.Printf("Found %d upcoming/active markets\n", len(upcoming))

	// Step 2: process markets for order placement
	for _, m := range upcoming {
		if b.ordersPlaced[m.ConditionID] {
			continue
		}
		if !shouldPlaceOrders(b.cfg, m, now) {
			continue
		}
		// Mirror python: skip placing if bot has active work in another market.
		if hasWork, reason := b.hasActiveMarketWork(ctx, now); hasWork {
			logger.Printf("Skipping %s - bot is %s\n", m.MarketSlug, reason)
			continue
		}
		logger.Printf("Placing orders for %s (starts in %.1f minutes)\n", m.MarketSlug, m.TimeUntilStart(now).Minutes())
		var (
			orders []models.OrderRecord
			err    error
		)
		switch strings.ToLower(strings.TrimSpace(b.cfg.OrderMode)) {
		case "liquidity":
			orders, err = b.placeLiquidityOrders(ctx, m)
		case "split":
			// Split策略：先split，然后根据盘口不均衡挂单套利
			config := DefaultSplitStrategyConfig()
			orders, err = b.executeSplitStrategy(ctx, m, config)
		default:
			orders, err = b.placeSimpleTestOrders(ctx, m, 0.49, 10.0)
		}
		if err != nil {
			b.recordError(err)
			continue
		}
		if len(orders) > 0 {
			b.ordersPlaced[m.ConditionID] = true
			b.activeOrders[m.ConditionID] = orders
			for _, o := range orders {
				b.orderHistory[o.OrderID] = o
			}
			_ = b.saveOrders()
			_ = b.saveOrderHistory()
		}
	}

	// Step 3: check active orders
	b.checkActiveOrders(ctx)

	// Step 3.5: strategy timeout exit (cancel + merge + sell leftovers)
	b.checkStrategyExecution(ctx, now)

	// Step 3.6: fallback orders if idle (python parity)
	if strings.ToLower(strings.TrimSpace(b.cfg.OrderMode)) == "liquidity" {
		// For liquidity mode, fallback means placing liquidity orders too.
		b.placeFallbackLiquidityIfIdle(ctx, upcoming, now)
	} else {
		b.placeFallbackOrdersIfIdle(ctx, upcoming, now)
	}

	// Step 5: cleanup old markets (>24h) (python parity)
	b.cleanupOldMarkets(ctx, now)

	// Step 4: refresh balance
	bal, err := b.chain.USDCBalance(ctx)
	if err == nil {
		b.mu.Lock()
		b.state.USDCBalance = bal
		b.mu.Unlock()
	}

	// Update state.total_pnl from order history (best-effort, parity with python)
	totalPNL := 0.0
	for _, o := range b.orderHistory {
		if o.PNLUSD != nil {
			totalPNL += *o.PNLUSD
		}
	}
	b.mu.Lock()
	b.state.TotalPNL = totalPNL
	b.mu.Unlock()

	b.updateOrderLists()
}

func (b *Bot) filterUpcoming(markets []models.Market, now time.Time) []models.Market {
	var out []models.Market
	nowTs := now.Unix()
	changed := false
	for _, m := range markets {
		if m.IsResolved {
			continue
		}
		timeUntilStart := m.StartTS - nowTs
		timeUntilEnd := m.EndTS - nowTs
		if timeUntilEnd > -300 && timeUntilStart <= 86400 {
			out = append(out, m)
			if _, ok := b.trackedMarkets[m.ConditionID]; !ok {
				b.trackedMarkets[m.ConditionID] = m
				b.ordersPlaced[m.ConditionID] = false
				changed = true
			}
		}
	}
	if changed {
		_ = b.saveMarkets()
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartTS < out[j].StartTS })
	return out
}

func shouldPlaceOrders(cfg config.Config, m models.Market, now time.Time) bool {
	sec := m.TimeUntilStart(now).Seconds()
	minS := float64(cfg.OrderPlacementMinMinutes * 60)
	maxS := float64(cfg.OrderPlacementMaxMinutes * 60)
	return sec >= minS && sec <= maxS
}

func (b *Bot) placeSimpleTestOrders(ctx context.Context, market models.Market, price float64, size float64) ([]models.OrderRecord, error) {
	// Balance check (best-effort)
	bal, _ := b.chain.USDCBalance(ctx)
	required := price * size * 2
	if bal > 0 && bal < required {
		return nil, fmt.Errorf("insufficient balance: $%.2f < $%.2f", bal, required)
	}

	yes, no := findYesNoOutcomes(market.Outcomes)
	if yes == nil || no == nil {
		return nil, errors.New("could not find both outcomes (Yes/No or Up/Down)")
	}

	var placed []models.OrderRecord
	for _, outcome := range []models.Outcome{*yes, *no} {
		ord, err := b.placeSingleFixed(ctx, market, outcome, price, size, models.OrderSideBuy)
		if err != nil {
			// record a failed order
			msg := err.Error()
			rec := models.OrderRecord{
				OrderID:         "FAILED",
				MarketSlug:      market.MarketSlug,
				ConditionID:     market.ConditionID,
				TokenID:         outcome.TokenID,
				Outcome:         outcome.Outcome,
				Side:            models.OrderSideBuy,
				Price:           price,
				Size:            0,
				SizeUSD:         price * size,
				Status:          models.OrderStatusFailed,
				CreatedAt:       time.Now(),
				ErrorMessage:    &msg,
				TransactionType: "BUY",
				CostUSD:         floatPtr(price * size),
				RevenueUSD:      floatPtr(0),
				PNLUSD:          floatPtr(-(price * size)),
			}
			placed = append(placed, rec)
			continue
		}
		placed = append(placed, ord)
		time.Sleep(500 * time.Millisecond)
	}
	return placed, nil
}

func (b *Bot) placeSingleFixed(ctx context.Context, market models.Market, outcome models.Outcome, price float64, size float64, side models.OrderSide) (models.OrderRecord, error) {
	if b.clob == nil {
		return models.OrderRecord{}, errors.New("clob client not initialized")
	}
	if b.clob.Address() == "" {
		return models.OrderRecord{}, errors.New("wallet address not available")
	}
	if side != models.OrderSideBuy {
		return models.OrderRecord{}, errors.New("only BUY implemented in Go port test strategy")
	}
	orderArgs := clob.OrderArgs{
		TokenID:    outcome.TokenID,
		Price:      price,
		Size:       size,
		Side:       clob.OrderSideBuy,
		FeeRateBps: 0,
		Nonce:      0,
		Expiration: 0,
		Taker:      "",
	}

	signed, _, err := b.clob.CreateOrder(ctx, orderArgs, nil, nil)
	if err != nil {
		return models.OrderRecord{}, err
	}
	resp, err := b.clob.PostOrder(ctx, signed, clob.OrderTypeGTC)
	if err != nil {
		return models.OrderRecord{}, err
	}
	orderID := asString(resp["orderID"])
	if orderID == "" {
		// fallback: salt
		orderID = fmt.Sprintf("%d", signed.Salt)
	}

	sizeUSD := price * size
	cost := sizeUSD
	pnl := -sizeUSD
	strategy := b.cfg.StrategyName
	return models.OrderRecord{
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
		CreatedAt:       time.Now(),
		Strategy:        &strategy,
		TransactionType: "BUY",
		CostUSD:         &cost,
		RevenueUSD:      floatPtr(0),
		PNLUSD:          &pnl,
	}, nil
}

func (b *Bot) checkActiveOrders(ctx context.Context) {
	changed := false
	for cid, orders := range b.activeOrders {
		market, hasMarket := b.trackedMarkets[cid]
		if !hasMarket {
			// Orphaned group: refresh statuses and potentially clear.
			ch, kept := b.refreshOrphanedOrders(ctx, cid, orders)
			if ch {
				changed = true
			}
			if kept == nil {
				continue
			}
			orders = kept

			// Best-effort: attempt periodic merge for orphaned orders, then mark sold when cleared.
			if !b.positionsSold[cid] {
				last := b.lastMergeAttempt[cid]
				if last.IsZero() || time.Since(last) >= 30*time.Second {
					stub := b.buildOrphanMarket(cid, orders)
					merged := b.mergePositionsIfPossible(ctx, stub, orders)
					if merged > 0 {
						b.trackMerge(stub, merged)
						changed = true
					}
					b.lastMergeAttempt[cid] = time.Now()
				}
				if cleared, known := b.walletPositionsCleared(ctx, cid, orders); known && cleared {
					b.positionsSold[cid] = true
					changed = true
				}
			}
			b.activeOrders[cid] = orders
			continue
		}
		for i := range orders {
			o := orders[i]
			if o.Status != models.OrderStatusPlaced && o.Status != models.OrderStatusPartiallyFilled {
				continue
			}
			details, err := b.clob.GetOrder(ctx, o.OrderID)
			if err != nil {
				continue
			}
			status := strings.ToUpper(asString(details["status"]))
			sizeMatched := asFloat(details["size_matched"])
			origSize := asFloat(details["original_size"])
			if origSize == 0 {
				origSize = o.Size
			}
			o.SizeMatched = &sizeMatched

			origStatus := o.Status
			switch {
			case status == "MATCHED" || (origSize > 0 && sizeMatched >= origSize):
				o.Status = models.OrderStatusFilled
				now := time.Now()
				o.FilledAt = &now
			case sizeMatched > 0:
				o.Status = models.OrderStatusPartiallyFilled
			case status == "CANCELLED":
				o.Status = models.OrderStatusCancelled
			case status == "OPEN" || status == "PLACED" || status == "LIVE" || status == "ACTIVE":
				o.Status = models.OrderStatusPlaced
			}
			if o.Status != origStatus {
				changed = true
			}
			orders[i] = o
			b.orderHistory[o.OrderID] = o
		}

		// Periodic merge while market is active (every ~30s)
		if hasMarket && !b.positionsSold[cid] {
			last := b.lastMergeAttempt[cid]
			if last.IsZero() || time.Since(last) >= 30*time.Second {
				merged := b.mergePositionsIfPossible(ctx, market, orders)
				if merged > 0 {
					b.trackMerge(market, merged)
					changed = true
				}
				b.lastMergeAttempt[cid] = time.Now()
			}

			// Sell leftovers 1 minute before end
			b.sellRemainingPositionsIfNeeded(ctx, market, orders)
		}

		// Cancel remaining open orders after market end (+5m)
		if hasMarket && time.Now().Unix() > market.EndTS+300 {
			for i := range orders {
				if orders[i].Status == models.OrderStatusPlaced || orders[i].Status == models.OrderStatusPartiallyFilled {
					_, _ = b.clob.Cancel(ctx, orders[i].OrderID)
					orders[i].Status = models.OrderStatusCancelled
					changed = true
					b.orderHistory[orders[i].OrderID] = orders[i]
				}
			}
			b.positionsSold[cid] = true
		}
		b.activeOrders[cid] = orders
	}
	if changed {
		_ = b.saveOrders()
		_ = b.saveOrderHistory()
	}
}

func (b *Bot) updateOrderLists() {
	all := make([]models.OrderRecord, 0)
	for _, orders := range b.activeOrders {
		all = append(all, orders...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })

	nowTs := time.Now().Unix()
	pending := make([]models.OrderRecord, 0)
	for _, o := range all {
		if o.Status == models.OrderStatusPlaced || o.Status == models.OrderStatusPartiallyFilled {
			// only if market still active
			m, ok := b.trackedMarkets[o.ConditionID]
			if !ok {
				pending = append(pending, o)
				continue
			}
			if m.EndTS >= nowTs-300 && !m.IsResolved {
				pending = append(pending, o)
			}
		}
	}

	hist := make([]models.OrderRecord, 0, len(b.orderHistory))
	for _, o := range b.orderHistory {
		hist = append(hist, o)
	}
	sort.Slice(hist, func(i, j int) bool { return hist[i].CreatedAt.After(hist[j].CreatedAt) })
	if len(hist) > 100 {
		hist = hist[:100]
	}

	b.mu.Lock()
	b.state.PendingOrders = pending
	b.state.RecentOrders = hist
	b.mu.Unlock()
}

func (b *Bot) recordError(err error) {
	msg := err.Error()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state.ErrorCount++
	b.state.LastError = &msg
}

func floatPtr(v float64) *float64 { return &v }

func findYesNoOutcomes(outs []models.Outcome) (*models.Outcome, *models.Outcome) {
	var yes, no *models.Outcome
	for i := range outs {
		u := strings.ToUpper(strings.TrimSpace(outs[i].Outcome))
		if (u == "YES" || u == "UP") && yes == nil {
			yes = &outs[i]
		}
		if (u == "NO" || u == "DOWN") && no == nil {
			no = &outs[i]
		}
	}
	return yes, no
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprintf("%v", v)
	}
}

func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		// best-effort
		var f float64
		_, _ = fmt.Sscanf(t, "%f", &f)
		return f
	default:
		return 0
	}
}

func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.ToLower(t) == "true"
	default:
		return false
	}
}
