# Architecture Documentation

This document describes the technical architecture of the Polymarket Limit Order Bot.

## System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      User Interface                         │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │         Web Dashboard (FastAPI + HTML)               │  │
│  │  - Real-time market monitoring                       │  │
│  │  - Order tracking                                    │  │
│  │  - Balance & PnL display                            │  │
│  │  - Live logs                                         │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                            ↕ HTTP API
┌─────────────────────────────────────────────────────────────┐
│                      Bot Core Logic                         │
│                                                             │
│  ┌────────────────┐  ┌──────────────┐  ┌────────────────┐ │
│  │ Market         │  │ Order        │  │ Bot Main       │ │
│  │ Discovery      │→ │ Manager      │← │ Loop           │ │
│  │                │  │              │  │                │ │
│  │ - Scan markets │  │ - Place      │  │ - Scheduling   │ │
│  │ - Filter BTC   │  │ - Monitor    │  │ - State mgmt   │ │
│  │ - Track timing │  │ - Cancel     │  │ - Cleanup      │ │
│  └────────────────┘  └──────────────┘  └────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                ↕                    ↕
┌─────────────────────┐   ┌──────────────────────────────────┐
│   Gamma API         │   │   CLOB API (py-clob-client)      │
│                     │   │                                  │
│ - Market metadata   │   │ - Order placement                │
│ - Event discovery   │   │ - Orderbook data                 │
│ - Market status     │   │ - Balance checks                 │
└─────────────────────┘   │ - Order status                   │
                          └──────────────────────────────────┘
                                      ↕
                          ┌──────────────────────────────────┐
                          │   Polygon Blockchain             │
                          │                                  │
                          │ - USDC transfers                 │
                          │ - CTF token operations           │
                          │ - Smart contract interactions    │
                          └──────────────────────────────────┘
```

## Core Components

### 1. Configuration Layer ([config.py](config.py))

**Purpose:** Centralized configuration management

**Key Features:**
- Loads settings from .env file
- Validates required parameters
- Type conversion and defaults
- Environment-aware configuration

**Configuration Categories:**
- Wallet authentication (private key, signature type)
- Trading parameters (order size, spread offset)
- API endpoints (Gamma, CLOB)
- Timing parameters (check interval, order placement timing)
- Dashboard settings (host, port)
- Logging configuration

### 2. Data Models Layer ([models.py](models.py))

**Purpose:** Strongly-typed data structures using Pydantic

**Key Models:**

#### Market
- Represents a BTC 15-minute market
- Tracks condition_id, slug, question
- Contains start/end timestamps
- Holds outcome data (Yes/No tokens)
- Computed properties: time_until_start, should_place_orders

#### OrderRecord
- Tracks individual limit orders
- Contains order_id, market info, token_id
- Records price, size, side (BUY/SELL)
- Tracks status (PENDING → PLACED → FILLED/CANCELLED)
- Timestamps for created_at and filled_at

#### BotState
- Aggregates current bot state
- Tracks active markets and orders
- Maintains balance and PnL
- Error tracking

### 3. Market Discovery ([market_discovery.py](market_discovery.py))

**Purpose:** Scan and identify BTC 15-minute markets

**Flow:**
1. Query Gamma API for events/markets
2. Filter for Bitcoin-related events
3. Extract market metadata
4. Parse timestamps from slug or metadata
5. Validate 15-minute duration
6. Return sorted list of markets

**Identification Logic:**
- Slug pattern: `btc-updown-15m-{unix_timestamp}`
- Question keywords: "Bitcoin Up or Down" + "15 minute"
- Duration validation: 850-950 seconds (15 min ± tolerance)

**Data Extraction:**
- Condition ID and market slug
- Question text
- Start/end timestamps
- Outcome tokens (Yes/No)
- Market status (active/resolved)

### 4. Order Management ([order_manager.py](order_manager.py))

**Purpose:** Interface with Polymarket CLOB for order operations

**Initialization:**
1. Create CLOB client with private key
2. Derive wallet address
3. Set USDC allowance
4. Set CTF exchange allowance

**Order Placement Strategy:**
```python
For each outcome (Yes/No):
  Buy price = best_bid - spread_offset
  Sell price = best_ask + spread_offset
  Size = order_size_usd / price

Validation:
  - Price clamped to [0.01, 0.99]
  - Price rounded to 0.01 tick size
  - Balance check before placement
  - Size validation
```

**Order Lifecycle:**
1. PENDING: Created but not submitted
2. PLACED: Successfully submitted to CLOB
3. PARTIALLY_FILLED: Some size matched
4. FILLED: Fully matched
5. CANCELLED: Manually cancelled
6. FAILED: Submission error

**Monitoring:**
- Periodic status checks via CLOB API
- Track size_matched vs original_size
- Update status and filled_at timestamp
- Log all status changes

### 5. Bot Main Loop ([bot.py](bot.py))

**Purpose:** Orchestrate market discovery, order placement, and monitoring

**Main Loop (run_once):**
```python
1. Discover markets
   ↓
2. Filter upcoming markets (next 24h)
   ↓
3. For each market:
   - Check if orders already placed
   - Check if it's placement time (5 min before)
   - Place liquidity orders if ready
   ↓
4. Check active order status
   ↓
5. Cancel unfilled orders after market ends
   ↓
6. Cleanup old markets (>24h old)
   ↓
7. Update bot state
   ↓
8. Sleep for CHECK_INTERVAL_SECONDS
```

**State Management:**
- tracked_markets: Dict[condition_id → Market]
- orders_placed: Dict[condition_id → bool]
- active_orders: Dict[condition_id → List[OrderRecord]]

**Thread Safety:**
- Uses threading.Lock for state access
- Provides thread-safe get_state() method
- Enables concurrent dashboard updates

### 6. Dashboard ([dashboard.py](dashboard.py))

**Purpose:** Real-time web interface for monitoring

**FastAPI Endpoints:**

| Endpoint | Purpose |
|----------|---------|
| `GET /` | Main dashboard HTML |
| `GET /api/status` | Bot status, balance, counts |
| `GET /api/markets` | Active markets with countdowns |
| `GET /api/orders` | Pending and recent orders |
| `GET /api/logs` | Recent log entries |

**Dashboard Features:**
- Auto-refresh every 5 seconds
- Real-time countdowns to market starts
- Color-coded order status badges
- Live log streaming
- Responsive grid layout

**Background Bot Integration:**
- Bot runs in daemon thread
- Dashboard spawns bot on startup
- Shared bot instance for state access
- Non-blocking state queries

### 7. Logging System ([logger.py](logger.py))

**Purpose:** Comprehensive logging for debugging and monitoring

**Configuration:**
- Dual handlers: file + console
- File: DEBUG level (all logs to bot.log)
- Console: INFO level (important logs only)
- Structured format with timestamps

**Log Levels:**
- DEBUG: Orderbook updates, state changes
- INFO: Market discovery, order placement
- WARNING: API issues, low balance
- ERROR: Exceptions, failures

## Data Flow

### Order Placement Flow

```
Market Start Time: 12:00:00
Current Time:      11:55:00 (5 min before)

1. Bot Loop Iteration
   ├─ discover_btc_15m_markets()
   │  └─ Returns: [Market(start=12:00:00, ...)]
   │
   ├─ market.should_place_orders == True
   │  └─ (time_until_start = 300s = 5 min)
   │
   ├─ update_market_prices()
   │  ├─ get_order_book(token_yes)
   │  │  └─ best_bid=0.50, best_ask=0.52
   │  └─ get_order_book(token_no)
   │     └─ best_bid=0.48, best_ask=0.50
   │
   └─ place_liquidity_orders()
      ├─ Outcome: Yes
      │  ├─ BUY @ 0.49 (0.50 - 0.01)
      │  │  └─ Size: 10/0.49 = 20.41 shares
      │  └─ SELL @ 0.53 (0.52 + 0.01)
      │     └─ Size: 10/0.53 = 18.87 shares
      │
      └─ Outcome: No
         ├─ BUY @ 0.47 (0.48 - 0.01)
         │  └─ Size: 10/0.47 = 21.28 shares
         └─ SELL @ 0.51 (0.50 + 0.01)
            └─ Size: 10/0.51 = 19.61 shares

Result: 4 orders placed, $40 total liquidity provided
```

### Order Monitoring Flow

```
Every 60 seconds:

1. For each active order:
   ├─ get_order(order_id)
   │  └─ Returns: {status, size_matched, original_size}
   │
   ├─ Check status
   │  ├─ status == "MATCHED" → OrderStatus.FILLED
   │  ├─ size_matched > 0 → OrderStatus.PARTIALLY_FILLED
   │  └─ status == "CANCELLED" → OrderStatus.CANCELLED
   │
   └─ Update order record
      └─ Log status change

After market ends (12:15:00 + 5min grace):
   └─ cancel_orders(unfilled_orders)
```

## Threading Model

```
Main Thread
├─ FastAPI/Uvicorn Server
│  ├─ Handles HTTP requests
│  ├─ Renders dashboard
│  └─ Returns JSON responses
│
└─ Bot Thread (daemon)
   ├─ Runs bot loop continuously
   ├─ Sleeps between iterations
   └─ Updates shared state (with lock)

Shared State Access:
   Dashboard ←[read]→ bot.get_state() ←[lock]→ Bot Loop
```

## Error Handling Strategy

### Graceful Degradation

1. **API Failures:**
   - Log error
   - Increment error counter
   - Continue to next iteration
   - Don't crash bot

2. **Order Placement Failures:**
   - Create OrderRecord with FAILED status
   - Store error message
   - Log detailed exception
   - Continue with other orders

3. **Balance Checks:**
   - Check before every order
   - Log warning if insufficient
   - Skip order placement
   - Continue monitoring

### Retry Logic

- No automatic retries for order placement (avoid duplicates)
- Transient API errors handled by next iteration
- Failed orders logged for manual review

## Security Considerations

1. **Private Key Storage:**
   - Loaded from .env (not committed)
   - Never logged
   - Used only for CLOB client init

2. **Allowances:**
   - Set programmatically with reasonable limits
   - USDC: $10,000 allowance
   - CTF exchange: Standard allowance

3. **Order Validation:**
   - Price bounds enforced [0.01, 0.99]
   - Size validation
   - Balance checks
   - No arbitrary order execution

## Performance Optimization

1. **Efficient Market Filtering:**
   - Filter by timestamp before detailed processing
   - Early exit for resolved markets
   - Cache tracked markets in memory

2. **Parallel Order Placement:**
   - Small delays between orders (0.5s)
   - Prevents rate limiting
   - Maintains order reliability

3. **Dashboard Optimization:**
   - Limit recent orders to 50
   - Aggregate data before sending
   - Auto-refresh at 5s intervals

## Extensibility

### Adding New Market Types

1. Create new discovery method in [market_discovery.py](market_discovery.py)
2. Add filtering logic
3. Register in bot loop

### Custom Order Strategies

1. Modify price calculation in `_adjust_price()`
2. Add new parameters to [config.py](config.py)
3. Update order placement logic

### Additional Monitoring

1. Add new endpoint to [dashboard.py](dashboard.py)
2. Create corresponding HTML section
3. Update auto-refresh logic

## Testing Strategy

### Unit Tests (Recommended)

```python
# Test market identification
def test_is_btc_15m_market():
    market = Market(slug="btc-updown-15m-123456", ...)
    assert discovery._is_btc_15m_market(market) == True

# Test order price calculation
def test_calculate_order_size():
    size = manager.calculate_order_size(price=0.50, usd=10.0)
    assert size == 20.0

# Test timing logic
def test_should_place_orders():
    market = Market(start_timestamp=now + 300)  # 5 min
    assert market.should_place_orders == True
```

### Integration Tests

```python
# Test Gamma API connection
python test_connection.py

# Test CLOB client initialization
# Test balance retrieval
# Test orderbook fetching
```

### Manual Testing

1. Start with `--check-config`
2. Run `test_connection.py`
3. Monitor first market cycle
4. Verify dashboard updates
5. Check order placement timing

## Deployment Considerations

### Production Checklist

- [ ] Set appropriate ORDER_SIZE_USD
- [ ] Fund wallet with sufficient USDC
- [ ] Test with small amounts first
- [ ] Monitor first few market cycles
- [ ] Set up log rotation
- [ ] Configure firewall for dashboard port
- [ ] Use process manager (systemd, pm2)
- [ ] Set up monitoring/alerting

### Recommended Setup

```bash
# Use screen/tmux for persistent session
screen -S polymarket-bot
python main.py

# Detach: Ctrl+A, D
# Reattach: screen -r polymarket-bot
```

Or use systemd service (Linux):

```ini
[Unit]
Description=Polymarket Limit Order Bot
After=network.target

[Service]
Type=simple
User=your-user
WorkingDirectory=/path/to/bot
ExecStart=/path/to/venv/bin/python main.py
Restart=always

[Install]
WantedBy=multi-user.target
```

## Monitoring and Maintenance

### Key Metrics to Track

- Order fill rate
- Time to fill
- USDC balance trend
- Error rate
- Market discovery success rate

### Regular Maintenance

- Review logs weekly
- Monitor USDC balance
- Check for API changes
- Update dependencies
- Review filled orders

### Debugging Tips

1. Check `bot.log` for detailed traces
2. Use dashboard for real-time state
3. Run `test_connection.py` for connectivity
4. Verify wallet balance on Polygonscan
5. Check Polymarket.com for market status

---

This architecture provides a robust, maintainable foundation for automated Polymarket trading with clear separation of concerns and extensibility for future enhancements.
