# Quick Start Guide

Get your Polymarket bot running in 5 minutes!

## Step 1: Prerequisites

Make sure you have:
- ✅ Python 3.9+ installed (`python --version`)
- ✅ A Polygon wallet with some USDC (minimum $50 recommended)
- ✅ Your wallet's private key

## Step 2: Installation

### Windows
```cmd
# Double-click run.bat or run in terminal:
run.bat
```

### Mac/Linux
```bash
# Make executable and run:
chmod +x run.sh
./run.sh
```

### Manual Installation
```bash
# Create virtual environment
python -m venv venv

# Activate it
# Windows:
venv\Scripts\activate
# Mac/Linux:
source venv/bin/activate

# Install dependencies
pip install -r requirements.txt
```

## Step 3: Configuration

1. Copy the example config:
```bash
cp .env.example .env
```

2. Edit `.env` with your favorite text editor:
```env
PRIVATE_KEY=your_private_key_here_without_0x_prefix
ORDER_SIZE_USD=10.0
SPREAD_OFFSET=0.01
```

**Important Notes:**
- Remove `0x` prefix from your private key if present
- Start with small ORDER_SIZE_USD for testing (e.g., 10)
- Never share your .env file or commit it to git

## Step 4: Test Connection

Run the connection test to verify everything works:

```bash
python test_connection.py
```

You should see:
```
✓ Configuration loaded successfully
✓ Successfully connected to Gamma API
✓ CLOB client initialized
✓ Successfully connected to CLOB API
```

## Step 5: Run the Bot

Start the bot with dashboard:

```bash
python main.py
```

Or use the run scripts:
```bash
# Windows
run.bat

# Mac/Linux
./run.sh
```

## Step 6: Access Dashboard

Open your browser and go to:
```
http://localhost:8000
```

You should see:
- Bot status and USDC balance
- Upcoming BTC 15-minute markets
- Active and recent orders
- Real-time logs

## What Happens Next?

The bot will:

1. **Scan for markets** every 60 seconds
2. **Find BTC 15m markets** starting in the next 24 hours
3. **Wait until 5 minutes before** each market starts
4. **Place 4 limit orders** (buy/sell on Yes/No outcomes)
5. **Monitor fills** and cancel unfilled orders after market ends

## Expected Behavior

### First Run (No Markets)
If no BTC 15m markets are found:
```
Discovering BTC 15-minute markets...
Found 0 upcoming/active markets
Sleeping for 60 seconds...
```

This is normal! The bot will keep checking.

### When Markets Are Found
```
Found 2 upcoming/active markets
Tracking new market: btc-updown-15m-1234567890
  (starts in 45.3 minutes)
```

### Order Placement Time
```
Placing orders for btc-updown-15m-1234567890
  (starts in 4.8 minutes)
Successfully placed 4 orders
  - BUY Yes @ $0.49 x 20.41 shares
  - SELL Yes @ $0.51 x 19.61 shares
  - BUY No @ $0.49 x 20.41 shares
  - SELL No @ $0.51 x 19.61 shares
```

## Troubleshooting

### "Configuration error: PRIVATE_KEY is required"
- Make sure you created `.env` file
- Check that PRIVATE_KEY is set correctly

### "Insufficient balance"
- Add more USDC to your wallet
- Reduce ORDER_SIZE_USD in .env

### "No markets found"
- This is normal if no BTC 15m markets exist
- Bot will keep checking automatically
- Check Polymarket.com to see if markets are available

### Orders not filling
- This is normal - not all orders fill
- Adjust SPREAD_OFFSET to be more competitive
- Orders are automatically cancelled after market ends

## Stopping the Bot

Press `Ctrl+C` in the terminal running the bot.

The bot will:
- Log shutdown request
- Keep existing orders active (they won't be cancelled)
- Exit gracefully

## Monitoring

While running, monitor:
- **Dashboard** (http://localhost:8000) - Real-time view
- **Terminal** - Live logs
- **bot.log** - Detailed log file

## Safety Tips

✅ **DO:**
- Start with small ORDER_SIZE_USD (e.g., $10)
- Monitor the bot regularly
- Keep your .env file secure
- Check your wallet balance

❌ **DON'T:**
- Commit .env to git
- Share your private key
- Run with large amounts without testing
- Leave bot unattended for days

## Next Steps

Once comfortable:
- Adjust ORDER_SIZE_USD for your strategy
- Tune SPREAD_OFFSET for better fill rates
- Monitor profitability in dashboard
- Read README.md for advanced configuration

## Getting Help

1. Check bot.log for detailed errors
2. Review dashboard for status
3. Re-run test_connection.py
4. Check README.md for more details

## Summary Checklist

- [ ] Python 3.9+ installed
- [ ] Wallet with USDC
- [ ] .env file configured
- [ ] test_connection.py passes
- [ ] Bot running
- [ ] Dashboard accessible
- [ ] Understanding expected behavior

Happy trading! 🚀

---

**Remember:** This is automated trading software. Always test with small amounts and understand the risks involved.
