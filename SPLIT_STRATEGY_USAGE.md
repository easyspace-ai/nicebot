# Split订单策略使用指南

## 概述

Split订单策略是一个专门为Polymarket BTC-15分钟市场设计的套利策略。该策略通过以下步骤实现：

1. **Split操作**：同时买入等量的UP和DOWN token（模拟split功能）
2. **套利挂单**：当盘口不均衡时（例如UP 53% DOWN 47%），通过挂单套利

## 策略原理

### Split操作
- 用10美金同时买入等量的UP和DOWN（例如各10 share）
- 理论上，UP和DOWN的价格应该都是0.50

### 套利机会
当盘口不均衡时：
- UP价格0.53，DOWN价格0.47（价差0.06）
- 可以在价格高的一边卖出，在价格低的一边买入
- 锁定价差利润，同时降低单边风险

## 配置参数

策略使用以下默认参数（可在代码中调整）：

```go
ImbalanceThreshold:           0.03  // 3%价差阈值
TradeRatio:                   0.4   // 交易40%的仓位
OrderOffset:                  0.01  // 1%的价格偏移
MinImbalance:                 0.02  // 最小2%价差
StopTradingMinutesBeforeStart: 5    // 市场开始前5分钟停止
```

### 参数说明

- **ImbalanceThreshold**：当UP和DOWN的价差超过此值时，执行套利交易
- **TradeRatio**：执行交易的仓位比例（0.0-1.0），例如0.4表示交易40%的仓位
- **OrderOffset**：挂单价格在best_bid/best_ask基础上的偏移量
- **MinImbalance**：最小价差阈值，避免在价差太小时交易
- **StopTradingMinutesBeforeStart**：市场开始前停止交易的时间（分钟）

## 使用方法

### 1. 设置环境变量

在`.env`文件中设置：

```bash
ORDER_MODE=split
ORDER_SIZE_USD=10.0
```

### 2. 运行Bot

```bash
go run cmd/polymarket-bot/main.go run
```

或者使用编译后的二进制文件：

```bash
./polymarket-bot run
```

### 3. 策略执行流程

1. **市场发现**：Bot自动发现即将开始的BTC-15分钟市场
2. **Split操作**：在订单放置时间窗口内，执行split操作（同时买入等量UP和DOWN）
3. **价格监控**：持续监控UP和DOWN的订单簿价格
4. **套利挂单**：当价差超过阈值时，自动挂单套利
5. **订单管理**：跟踪订单状态，订单成交后更新仓位
6. **退出策略**：市场开始前5分钟停止交易，市场开始后merge剩余仓位

## 策略示例

### 场景1：UP价格偏高

**初始状态**：
- Split后：10 share UP + 10 share DOWN
- UP价格：0.53（best_bid: 0.52, best_ask: 0.54）
- DOWN价格：0.47（best_bid: 0.46, best_ask: 0.48）
- 价差：0.06（6%）

**策略执行**：
- 卖出4 share UP @ 0.51（best_bid - 0.01）
- 买入4.35 share DOWN @ 0.49（best_ask + 0.01）

**结果**：
- 收入：4 × 0.51 = 2.04 USDC
- 成本：4.35 × 0.49 = 2.13 USDC
- 净成本：0.09 USDC
- 剩余仓位：6 share UP + 14.35 share DOWN

**利润分析**：
- 如果市场回归均衡（都是0.50），可以merge剩余仓位
- Merge收益：6 × 0.50 + 14.35 × 0.50 = 10.175 USDC
- 总收益：10.175 - 10 - 0.09 = 0.085 USDC（0.85%）

### 场景2：DOWN价格偏高

**初始状态**：
- Split后：10 share UP + 10 share DOWN
- UP价格：0.47（best_bid: 0.46, best_ask: 0.48）
- DOWN价格：0.53（best_bid: 0.52, best_ask: 0.54）
- 价差：0.06（6%）

**策略执行**：
- 卖出4 share DOWN @ 0.51（best_bid - 0.01）
- 买入4.35 share UP @ 0.49（best_ask + 0.01）

**结果**：
- 收入：4 × 0.51 = 2.04 USDC
- 成本：4.35 × 0.49 = 2.13 USDC
- 净成本：0.09 USDC
- 剩余仓位：14.35 share UP + 6 share DOWN

## 风险提示

1. **Gas费用**：频繁交易会产生gas费用，需要确保利润 > gas费用
2. **滑点风险**：订单簿可能深度不够，实际成交价格可能偏离预期
3. **市场风险**：如果市场单边移动，可能产生损失
4. **流动性风险**：需要确保有足够的流动性来执行订单
5. **价差风险**：如果价差继续扩大，可能错过更大的套利机会

## 优化建议

1. **调整参数**：根据实际市场情况调整价差阈值和交易比例
2. **监控订单**：定期检查订单状态，及时调整策略
3. **风险管理**：设置最大交易比例，保留部分仓位对冲
4. **时间窗口**：在市场开始前适当的时间停止交易，避免市场波动风险

## 代码位置

- 策略实现：`internal/bot/split_strategy.go`
- 策略配置：`internal/bot/split_strategy.go` 中的 `DefaultSplitStrategyConfig()`
- Bot集成：`internal/bot/bot.go` 中的订单放置逻辑

## 调试和监控

1. **日志**：查看bot日志文件（默认：`bot.log`）
2. **Dashboard**：访问dashboard查看订单状态和市场信息
3. **订单历史**：检查`bot_orders.json`和`order_history.json`文件

## 常见问题

**Q: 为什么订单没有成交？**
A: 可能是价格设置不合理，或者订单簿深度不够。可以调整`OrderOffset`参数。

**Q: 如何调整交易比例？**
A: 修改`DefaultSplitStrategyConfig()`中的`TradeRatio`参数。

**Q: 策略什么时候停止交易？**
A: 默认在市场开始前5分钟停止交易，可以通过`StopTradingMinutesBeforeStart`参数调整。

**Q: 如何查看策略收益？**
A: 查看`order_history.json`文件中的`PNLUSD`字段，或者在dashboard中查看总收益。
