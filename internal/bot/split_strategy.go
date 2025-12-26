package bot

import (
	"context"
	"errors"
	"math"
	"time"

	"limitorderbot/internal/logging"
	"limitorderbot/internal/models"
)

// SplitOrderStrategyConfig 配置split订单策略的参数
type SplitOrderStrategyConfig struct {
	// 价差阈值：当UP和DOWN的价差超过此值时执行交易
	ImbalanceThreshold float64
	// 交易比例：执行交易的仓位比例（0.0-1.0）
	TradeRatio float64
	// 挂单偏移：在best_bid/best_ask基础上的价格偏移
	OrderOffset float64
	// 最小价差：避免在价差太小时交易
	MinImbalance float64
	// 停止交易时间：市场开始前X分钟停止交易
	StopTradingMinutesBeforeStart int
}

// DefaultSplitStrategyConfig 返回默认配置
func DefaultSplitStrategyConfig() SplitOrderStrategyConfig {
	return SplitOrderStrategyConfig{
		ImbalanceThreshold:           0.03,  // 3%价差
		TradeRatio:                   0.4,   // 交易40%的仓位
		OrderOffset:                  0.01,  // 1%的价格偏移
		MinImbalance:                 0.02,  // 最小2%价差
		StopTradingMinutesBeforeStart: 5,    // 市场开始前5分钟停止
	}
}

// placeSplitOrders 执行split订单策略
// 1. 通过CLOB同时买入等量的UP和DOWN（模拟split）
// 2. 检查盘口不均衡情况
// 3. 在不均衡时挂单套利
func (b *Bot) placeSplitOrders(ctx context.Context, market models.Market, config SplitOrderStrategyConfig) ([]models.OrderRecord, error) {
	logger := logging.Logger()

	if b.clob == nil {
		return nil, errors.New("clob client not initialized")
	}
	if b.clob.Address() == "" {
		return nil, errors.New("wallet address not available")
	}

	// 检查是否应该停止交易（市场开始前X分钟）
	now := time.Now()
	timeUntilStart := market.TimeUntilStart(now)
	if timeUntilStart.Minutes() < float64(config.StopTradingMinutesBeforeStart) {
		logger.Printf("Skipping split orders for %s - too close to market start (%.1f min)\n",
			market.MarketSlug, timeUntilStart.Minutes())
		return nil, nil
	}

	// 确保有价格数据
	market = b.fillMarketPrices(ctx, []models.Market{market})[0]

	yesOutcome, noOutcome := findYesNoOutcomes(market.Outcomes)
	if yesOutcome == nil || noOutcome == nil {
		return nil, errors.New("could not find both UP and DOWN outcomes")
	}

	// 检查是否有有效的订单簿数据
	if yesOutcome.BestBid == nil || yesOutcome.BestAsk == nil ||
		noOutcome.BestBid == nil || noOutcome.BestAsk == nil {
		logger.Printf("Insufficient orderbook data for %s\n", market.MarketSlug)
		return nil, errors.New("insufficient orderbook data")
	}

	// 计算中间价
	midUp := (*yesOutcome.BestBid + *yesOutcome.BestAsk) / 2
	midDown := (*noOutcome.BestBid + *noOutcome.BestAsk) / 2

	// 计算不均衡度
	imbalance := math.Abs(midUp - midDown)

	logger.Printf("Split strategy for %s: UP mid=%.4f, DOWN mid=%.4f, imbalance=%.4f\n",
		market.MarketSlug, midUp, midDown, imbalance)

	// 检查是否满足交易条件
	if imbalance < config.MinImbalance {
		logger.Printf("Imbalance %.4f below minimum threshold %.4f, skipping\n",
			imbalance, config.MinImbalance)
		return nil, nil
	}

	if imbalance < config.ImbalanceThreshold {
		logger.Printf("Imbalance %.4f below threshold %.4f, skipping\n",
			imbalance, config.ImbalanceThreshold)
		return nil, nil
	}

	// 计算交易数量（基于配置的订单大小）
	splitAmount := b.cfg.OrderSizeUSD
	tradeAmount := splitAmount * config.TradeRatio

	var orders []models.OrderRecord

	// 确定交易方向
	if midUp > midDown {
		// UP价格偏高，卖出UP，买入DOWN
		logger.Printf("UP price higher (%.4f > %.4f), selling UP and buying DOWN\n", midUp, midDown)

		// 卖出UP：在best_bid基础上减去偏移
		sellPrice := *yesOutcome.BestBid - config.OrderOffset
		if sellPrice < 0.01 {
			sellPrice = 0.01
		}
		sellSize := tradeAmount / sellPrice

		// 买入DOWN：在best_ask基础上加上偏移
		buyPrice := *noOutcome.BestAsk + config.OrderOffset
		if buyPrice > 0.99 {
			buyPrice = 0.99
		}
		buySize := tradeAmount / buyPrice

		// 获取tick size
		tickUp := b.getTickSize(ctx, yesOutcome.TokenID)
		tickDown := b.getTickSize(ctx, noOutcome.TokenID)

		// 调整价格到tick
		sellPrice = adjustPriceToTick(sellPrice, tickUp)
		buyPrice = adjustPriceToTick(buyPrice, tickDown)

		// 放置卖出UP订单
		if sellSize > 0.01 {
			order := b.placeSingleOrderBestEffort(ctx, market, *yesOutcome, models.OrderSideSell, sellPrice, sellSize)
			orders = append(orders, order)
			time.Sleep(500 * time.Millisecond)
		}

		// 放置买入DOWN订单
		if buySize > 0.01 {
			order := b.placeSingleOrderBestEffort(ctx, market, *noOutcome, models.OrderSideBuy, buyPrice, buySize)
			orders = append(orders, order)
			time.Sleep(500 * time.Millisecond)
		}

	} else if midDown > midUp {
		// DOWN价格偏高，卖出DOWN，买入UP
		logger.Printf("DOWN price higher (%.4f > %.4f), selling DOWN and buying UP\n", midDown, midUp)

		// 卖出DOWN：在best_bid基础上减去偏移
		sellPrice := *noOutcome.BestBid - config.OrderOffset
		if sellPrice < 0.01 {
			sellPrice = 0.01
		}
		sellSize := tradeAmount / sellPrice

		// 买入UP：在best_ask基础上加上偏移
		buyPrice := *yesOutcome.BestAsk + config.OrderOffset
		if buyPrice > 0.99 {
			buyPrice = 0.99
		}
		buySize := tradeAmount / buyPrice

		// 获取tick size
		tickUp := b.getTickSize(ctx, yesOutcome.TokenID)
		tickDown := b.getTickSize(ctx, noOutcome.TokenID)

		// 调整价格到tick
		sellPrice = adjustPriceToTick(sellPrice, tickDown)
		buyPrice = adjustPriceToTick(buyPrice, tickUp)

		// 放置卖出DOWN订单
		if sellSize > 0.01 {
			order := b.placeSingleOrderBestEffort(ctx, market, *noOutcome, models.OrderSideSell, sellPrice, sellSize)
			orders = append(orders, order)
			time.Sleep(500 * time.Millisecond)
		}

		// 放置买入UP订单
		if buySize > 0.01 {
			order := b.placeSingleOrderBestEffort(ctx, market, *yesOutcome, models.OrderSideBuy, buyPrice, buySize)
			orders = append(orders, order)
			time.Sleep(500 * time.Millisecond)
		}
	}

	if len(orders) > 0 {
		logger.Printf("Placed %d split strategy orders for %s\n", len(orders), market.MarketSlug)
		return b.verifyOrdersInOrderbook(ctx, market, orders), nil
	}

	return orders, nil
}

// getTickSize 获取token的tick size
func (b *Bot) getTickSize(ctx context.Context, tokenID string) float64 {
	if ts, err := b.clob.GetTickSize(ctx, tokenID); err == nil {
		if f, ok := parseTickSize(ts); ok && f > 0 {
			return f
		}
	}
	return 0.01 // 默认tick size
}

// executeSplitStrategy 执行完整的split策略流程
// 包括：1) split操作（同时买入等量UP和DOWN），2) 根据盘口不均衡挂单
func (b *Bot) executeSplitStrategy(ctx context.Context, market models.Market, config SplitOrderStrategyConfig) ([]models.OrderRecord, error) {
	logger := logging.Logger()
	logger.Printf("Executing split strategy for %s\n", market.MarketSlug)

	// Step 1: 执行split操作（通过CLOB同时买入等量的UP和DOWN）
	splitOrders, err := b.performSplit(ctx, market)
	if err != nil {
		logger.Printf("Split operation failed: %v\n", err)
		return nil, err
	}

	// Step 2: 等待一段时间让订单成交
	time.Sleep(2 * time.Second)

	// Step 3: 检查盘口不均衡并挂单
	arbitrageOrders, err := b.placeSplitOrders(ctx, market, config)
	if err != nil {
		logger.Printf("Split arbitrage orders failed: %v\n", err)
		// 不返回错误，因为split已经成功
	}

	// 合并所有订单
	allOrders := append(splitOrders, arbitrageOrders...)
	return allOrders, nil
}

// performSplit 执行split操作：同时买入等量的UP和DOWN
func (b *Bot) performSplit(ctx context.Context, market models.Market) ([]models.OrderRecord, error) {
	logger := logging.Logger()

	yesOutcome, noOutcome := findYesNoOutcomes(market.Outcomes)
	if yesOutcome == nil || noOutcome == nil {
		return nil, errors.New("could not find both UP and DOWN outcomes")
	}

	// 确保有价格数据
	market = b.fillMarketPrices(ctx, []models.Market{market})[0]

	// 计算split数量：用一半的资金买UP，一半买DOWN
	splitAmount := b.cfg.OrderSizeUSD / 2

	var orders []models.OrderRecord

	// 买入UP：使用best_ask价格
	if yesOutcome.BestAsk != nil && *yesOutcome.BestAsk > 0 {
		buyPrice := *yesOutcome.BestAsk
		buySize := splitAmount / buyPrice

		tick := b.getTickSize(ctx, yesOutcome.TokenID)
		buyPrice = adjustPriceToTick(buyPrice, tick)

		if buySize > 0.01 {
			order := b.placeSingleOrderBestEffort(ctx, market, *yesOutcome, models.OrderSideBuy, buyPrice, buySize)
			orders = append(orders, order)
			time.Sleep(500 * time.Millisecond)
		}
	}

	// 买入DOWN：使用best_ask价格
	if noOutcome.BestAsk != nil && *noOutcome.BestAsk > 0 {
		buyPrice := *noOutcome.BestAsk
		buySize := splitAmount / buyPrice

		tick := b.getTickSize(ctx, noOutcome.TokenID)
		buyPrice = adjustPriceToTick(buyPrice, tick)

		if buySize > 0.01 {
			order := b.placeSingleOrderBestEffort(ctx, market, *noOutcome, models.OrderSideBuy, buyPrice, buySize)
			orders = append(orders, order)
			time.Sleep(500 * time.Millisecond)
		}
	}

	if len(orders) > 0 {
		logger.Printf("Performed split: placed %d orders (UP + DOWN)\n", len(orders))
		return b.verifyOrdersInOrderbook(ctx, market, orders), nil
	}

	return nil, errors.New("no orders placed for split")
}
