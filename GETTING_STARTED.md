# Getting Started - Visual Guide

Complete visual walkthrough to get your Polymarket bot running.

## 🎯 What This Bot Does

```
┌─────────────────────────────────────────────────────────────┐
│                    YOUR POLYMARKET BOT                       │
│                                                              │
│  Automatically provides liquidity on BTC 15-minute markets  │
│                                                              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐ │
│  │   DISCOVERS  │ →  │    PLACES    │ →  │   MONITORS   │ │
│  │   Markets    │    │   Orders     │    │    Fills     │ │
│  └──────────────┘    └──────────────┘    └──────────────┘ │
│                                                              │
│  Every 60 seconds, looks for new BTC 15m markets            │
│  Places limit orders 5 minutes before market starts         │
│  Tracks order fills and cancels unfilled ones               │
└─────────────────────────────────────────────────────────────┘
```

## 📋 Pre-Installation Checklist

Before starting, make sure you have:

- [ ] **Python 3.9 or higher** installed
  ```bash
  python --version  # Should show 3.9.x or higher
  ```

- [ ] **A Polygon wallet** with:
  - [ ] Private key (64 hex characters, no 0x prefix)
  - [ ] **Minimum $50 USDC** on Polygon network
  - [ ] Wallet has been used on Polymarket at least once

- [ ] **Internet connection** (for API access)

- [ ] **Text editor** (VS Code, Notepad++, nano, etc.)

## 🚀 Installation Steps

### Step 1: Download/Clone the Code

If you have this folder, you're already done! Otherwise:

```bash
git clone <repository-url>
cd limitorderbot-claude
```

### Step 2: Set Up Python Environment

#### Option A: Using Run Scripts (Easiest)

**Windows:**
```cmd
run.bat
```

**Mac/Linux:**
```bash
chmod +x run.sh
./run.sh
```

These scripts automatically:
- Create virtual environment
- Install dependencies
- Start the bot

#### Option B: Manual Setup

```bash
# 1. Create virtual environment
python -m venv venv

# 2. Activate it
# Windows:
venv\Scripts\activate
# Mac/Linux:
source venv/bin/activate

# 3. Install dependencies
pip install -r requirements.txt
```

You should see:
```
✓ Successfully installed py-clob-client-0.25.0
✓ Successfully installed fastapi-0.104.0
✓ Successfully installed uvicorn-0.24.0
... (and more)
```

## ⚙️ Configuration

### Step 1: Create .env File

```bash
# Copy the example
cp .env.example .env
```

### Step 2: Edit .env File

Open `.env` in your text editor and fill in:

```env
# ============================================
# REQUIRED: Your wallet private key
# ============================================
PRIVATE_KEY=abc123...def456    # 64 characters, NO 0x prefix

# ============================================
# TRADING PARAMETERS
# ============================================
ORDER_SIZE_USD=10.0            # USD per order (start small!)
SPREAD_OFFSET=0.01             # Price offset ($0.01)

# ============================================
# ADVANCED (usually don't need to change)
# ============================================
CHAIN_ID=137                   # Polygon mainnet
SIGNATURE_TYPE=EOA             # Standard wallet
CHECK_INTERVAL_SECONDS=60      # How often to check
ORDER_PLACEMENT_MINUTES_BEFORE=5  # When to place orders
DASHBOARD_PORT=8000            # Web dashboard port
```

### Configuration Explanation

| Setting | What It Means | Example |
|---------|---------------|---------|
| `PRIVATE_KEY` | Your wallet's private key | `abc123def456...` (64 chars) |
| `ORDER_SIZE_USD` | $ amount per order | `10.0` = $10 per order |
| `SPREAD_OFFSET` | How far from best price | `0.01` = 1 cent offset |
| `CHECK_INTERVAL_SECONDS` | Check frequency | `60` = every minute |
| `ORDER_PLACEMENT_MINUTES_BEFORE` | Placement timing | `5` = 5 min before start |

### ⚠️ Important Notes

**About PRIVATE_KEY:**
- Get it from your wallet (MetaMask, etc.)
- Remove `0x` prefix if present
- Should be exactly 64 hexadecimal characters
- Never share this with anyone!
- Never commit .env to git (it's in .gitignore)

**About ORDER_SIZE_USD:**
- Bot places **4 orders per market**
- Total USDC needed = `4 × ORDER_SIZE_USD`
- Default `10.0` = $40 total per market
- Start small for testing!

**About SPREAD_OFFSET:**
- Smaller = more likely to fill (but less profit)
- Larger = less likely to fill (but more profit)
- `0.01` is a good starting point

## ✅ Test Your Setup

Before running the bot, test your configuration:

```bash
python test_connection.py
```

**Expected Output:**

```
╔════════════════════════════════════════════════════════════╗
║   Polymarket Limit Order Bot - Connection Test            ║
╚════════════════════════════════════════════════════════════╝

============================================================
CONFIGURATION TEST
============================================================
✓ Configuration loaded successfully
  - Chain ID: 137
  - Signature Type: EOA
  - Order Size: $10.0
  - Spread Offset: 0.01
  - Check Interval: 60s

============================================================
GAMMA API TEST
============================================================
✓ Market discovery client initialized
  Fetching markets...
✓ Successfully connected to Gamma API
  - Found 3 BTC 15m markets

  Recent markets:
    - btc-updown-15m-1234567890
      Start: 2025-12-19 12:00:00

============================================================
CLOB CLIENT TEST
============================================================
  Initializing CLOB client...
✓ CLOB client initialized
  - Wallet address: 0xYourAddress...
  Fetching USDC balance...
✓ Successfully connected to CLOB API
  - USDC Balance: $123.45

============================================================
TEST SUMMARY
============================================================
Configuration         ✓ PASSED
Gamma API            ✓ PASSED
CLOB Client          ✓ PASSED
============================================================

🎉 All tests passed! Your bot is ready to run.
```

### If Tests Fail

Check [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for solutions.

Common issues:
- Wrong PRIVATE_KEY format
- No internet connection
- Insufficient USDC balance

## 🎮 Running the Bot

### Start the Bot

```bash
python main.py
```

You should see:

```
============================================================
Starting Polymarket Limit Order Bot
============================================================
Wallet address: 0xYourAddress...
Order size: $10.0 per order
Spread offset: 0.01
Order placement: 5 min before start
============================================================
INFO - Discovering BTC 15-minute markets...
INFO - Found 2 upcoming/active markets
INFO - Tracking new market: btc-updown-15m-1234567890
INFO - Market btc-updown-15m-1234567890: 45.3 min until placement time
INFO - Starting dashboard on 0.0.0.0:8000
INFO - Sleeping for 60 seconds...
```

### Access the Dashboard

1. Open your browser
2. Go to: **http://localhost:8000**

You should see:

```
┌─────────────────────────────────────────────────────────┐
│  🤖 Polymarket Limit Order Bot                          │
│                                                          │
│  Status     USDC        Markets    Orders    Last Check │
│  ● Running  $123.45     2          0         12:34:56   │
├─────────────────────────────────────────────────────────┤
│  Upcoming BTC 15-Minute Markets                         │
│                                                          │
│  Market                    Starts       Countdown        │
│  btc-updown-15m-123456    14:00:00     45m 23s          │
│  btc-updown-15m-234567    14:15:00     1h 0m 23s        │
├─────────────────────────────────────────────────────────┤
│  Open Orders                                            │
│                                                          │
│  (No open orders yet)                                   │
├─────────────────────────────────────────────────────────┤
│  Recent Orders                                          │
│                                                          │
│  (No recent orders yet)                                 │
└─────────────────────────────────────────────────────────┘
```

## 📊 Understanding the Bot Behavior

### Phase 1: Market Discovery (First 60 seconds)

```
Bot Loop:
  ├─ Scan Gamma API
  ├─ Find BTC 15m markets
  └─ Show in dashboard

You'll see:
  "Found 2 upcoming/active markets"
```

### Phase 2: Waiting (Until 5 min before market)

```
Bot Loop:
  ├─ Check each market
  ├─ Calculate time until start
  └─ Wait if too early

You'll see:
  "Market btc-updown-15m-...: 45.3 min until placement time"

Dashboard shows:
  Countdown: 45m 23s → 44m 23s → 43m 23s ...
```

### Phase 3: Order Placement (5 min before start)

```
Bot detects it's time:
  ├─ Fetch current orderbook
  ├─ Calculate prices
  │  ├─ Buy price = best_bid - 0.01
  │  └─ Sell price = best_ask + 0.01
  ├─ Place 4 orders:
  │  ├─ BUY Yes @ $0.49
  │  ├─ SELL Yes @ $0.51
  │  ├─ BUY No @ $0.49
  │  └─ SELL No @ $0.51
  └─ Start monitoring

You'll see:
  "Placing orders for btc-updown-15m-1234567890"
  "Successfully placed 4 orders"
  "  - BUY Yes @ $0.49 x 20.41 shares"
  ...

Dashboard shows:
  Open Orders: 4
  Order details in table
```

### Phase 4: Monitoring (Until market ends)

```
Every 60 seconds:
  ├─ Check each order status
  ├─ Log fills
  └─ Update dashboard

You'll see:
  "Order abc123... filled completely"
  or
  "Order def456... partially filled: 10.5/20.0"

Dashboard shows:
  Order status: FILLED / PARTIALLY_FILLED / PLACED
```

### Phase 5: Cleanup (After market ends)

```
Market ends + 5 min grace:
  ├─ Find unfilled orders
  ├─ Cancel them
  └─ Remove from active tracking

You'll see:
  "Cancelling 2 unfilled orders for btc-updown-15m-..."
  "Order cancelled: xyz789..."
```

## 🎯 First Run Expectations

### Scenario 1: No Markets Available

```
Found 0 upcoming/active markets
Sleeping for 60 seconds...
```

**This is normal!** BTC 15m markets aren't always available.

**What to do:**
- Let bot keep running
- It will check every 60 seconds
- Check Polymarket.com for available markets

### Scenario 2: Markets Found But Far Away

```
Found 1 upcoming/active markets
Market btc-updown-15m-...: 120.5 min until placement time
Sleeping for 60 seconds...
```

**This is normal!** Bot waits until 5 min before.

**What to do:**
- Leave bot running
- Check dashboard countdown
- Orders will be placed automatically

### Scenario 3: Markets Found, Orders Placed

```
Placing orders for btc-updown-15m-1234567890
Successfully placed 4 orders
  - BUY Yes @ $0.49 x 20.41 shares ($10.00)
  - SELL Yes @ $0.51 x 19.61 shares ($10.00)
  - BUY No @ $0.49 x 20.41 shares ($10.00)
  - SELL No @ $0.51 x 19.61 shares ($10.00)
```

**Success!** Bot is working.

**What to do:**
- Monitor dashboard
- Watch for fills
- Check your wallet on Polymarket

## 🛑 Stopping the Bot

### To stop the bot:

1. Go to terminal running the bot
2. Press **Ctrl+C**

```
^C
INFO - Received interrupt signal
INFO - Stopping bot...
INFO - Bot stopped
```

**Note:** Existing orders will remain active on Polymarket. They won't be automatically cancelled when you stop the bot.

### To cancel orders manually:

1. Visit Polymarket.com
2. Go to your open orders
3. Cancel manually

Or restart bot - it will resume monitoring and cancel old orders.

## 📈 Monitoring Your Bot

### Check Dashboard

Visit **http://localhost:8000** to see:
- Bot status
- USDC balance
- Upcoming markets with countdowns
- Open orders
- Recent order history
- Live logs

### Check Log File

```bash
# View entire log
cat bot.log

# Follow live logs
tail -f bot.log

# Search for errors
grep ERROR bot.log

# Search for specific market
grep "btc-updown-15m-1234567890" bot.log
```

### Check Your Wallet

- Visit [Polygonscan](https://polygonscan.com)
- Enter your wallet address
- View USDC transactions
- See order fills

## 🎓 Next Steps

Once you're comfortable:

1. **Tune Strategy:**
   - Adjust `ORDER_SIZE_USD` for your capital
   - Tune `SPREAD_OFFSET` for fill rate
   - Monitor profitability

2. **Scale Up:**
   - Start with $10 per order
   - Increase after successful cycles
   - Keep sufficient USDC balance

3. **Automate:**
   - Set up systemd service (Linux)
   - Use screen/tmux for persistence
   - Set up monitoring/alerts

4. **Learn More:**
   - Read [ARCHITECTURE.md](ARCHITECTURE.md) for technical details
   - Read [README.md](README.md) for full documentation
   - Check [TROUBLESHOOTING.md](TROUBLESHOOTING.md) if issues arise

## ❓ FAQ

**Q: How much USDC do I need?**
A: Minimum $50. Each market requires 4 × ORDER_SIZE_USD.

**Q: Will orders always fill?**
A: No. Depends on market liquidity and your spread. Some orders may not fill.

**Q: What happens if I stop the bot?**
A: Existing orders remain active. Bot state is lost. Restart to resume.

**Q: Can I run multiple bots?**
A: Yes, but use different wallets to avoid conflicts.

**Q: Is this profitable?**
A: Depends on market conditions, fills, and fees. Start small and monitor.

**Q: What are the risks?**
A: Market risk, execution risk, smart contract risk. Only trade what you can afford to lose.

## 🆘 Need Help?

1. Check [TROUBLESHOOTING.md](TROUBLESHOOTING.md)
2. Review logs in `bot.log`
3. Run `python test_connection.py`
4. Check dashboard at http://localhost:8000

## ✅ Quick Checklist

Before you start:
- [ ] Python 3.9+ installed
- [ ] Dependencies installed (`pip install -r requirements.txt`)
- [ ] `.env` file created and configured
- [ ] PRIVATE_KEY set correctly (no 0x prefix)
- [ ] Wallet has USDC on Polygon
- [ ] `test_connection.py` passes all tests
- [ ] You understand the bot behavior
- [ ] You're ready to monitor the first run

**You're ready! Run `python main.py` and open http://localhost:8000** 🚀

---

**Happy trading!** Remember to start small, monitor regularly, and understand the risks.
