# Polymarket Limit Order Bot

A production-ready automated trading bot that provides liquidity on Polymarket's Bitcoin Up or Down 15-minute binary outcome markets.

## Features

- **Automated Market Discovery**: Continuously scans for upcoming BTC 15-minute markets using the Gamma API
- **Strategic Order Placement**: Places two-sided limit orders (bid/ask on Yes/No outcomes) exactly 5 minutes before market start
- **Order Management**: Monitors order fills and automatically cancels unfilled orders after market resolution
- **Position Redemption**: Automatically redeem winning positions after market resolution
- **Real-time Dashboard**: Beautiful web interface showing markets, orders, balances, and logs
- **Robust Error Handling**: Comprehensive logging, retry logic, and balance checks
- **Flexible Configuration**: Easy setup via .env file

## How It Works

1. **Market Discovery**: Scans Gamma API for BTC 15-minute markets (identified by slug pattern `btc-updown-15m-*` or question keywords)
2. **Timing**: Tracks market start times and triggers order placement 5 minutes before the 15-minute window begins
3. **Order Placement**: For each outcome (Yes/No):
   - Buy limit order: `best_bid - spread_offset` (default: -$0.01)
   - Sell limit order: `best_ask + spread_offset` (default: +$0.01)
   - Order size: Configurable USD amount (default: $10 per order = 4 orders × $10 = $40 total)
4. **Monitoring**: Checks order status every 60 seconds to track fills
5. **Cleanup**: Cancels unfilled orders 5 minutes after market ends

## Quick Start

### Prerequisites

- Python 3.9 or higher
- Polygon wallet with USDC
- Private key for your wallet

### Installation

1. Clone this repository:
```bash
git clone <your-repo-url>
cd limitorderbot-claude
```

2. Install dependencies:
```bash
pip install -r requirements.txt
```

3. Create `.env` file from example:
```bash
cp .env.example .env
```

4. Edit `.env` and add your configuration:
```env
PRIVATE_KEY=your_private_key_here
ORDER_SIZE_USD=10.0
SPREAD_OFFSET=0.01
```

5. Verify configuration:
```bash
python main.py --check-config
```

### Running the Bot

**Option 1: Full mode (Bot + Dashboard)** - Recommended
```bash
python main.py
```
Then open http://localhost:8000 in your browser

**Option 2: Bot only** (no dashboard)
```bash
python main.py --mode bot
```

**Option 3: Dashboard only** (for testing)
```bash
python main.py --mode dashboard
```

### Redeeming Positions

After markets resolve, redeem your winning positions to get USDC back:

```bash
python redeem_positions.py
```

This script:
- Automatically fetches all your redeemable positions from Polymarket API
- Shows you the total value to be redeemed
- Asks for confirmation before submitting redemption transactions
- Returns USDC to your wallet by burning CTF tokens

**Manual redemption** (if you know the condition ID):
```bash
python redeem_with_condition_id.py
```

See [REDEMPTION_GUIDE.md](REDEMPTION_GUIDE.md) for detailed redemption information.

## Configuration

Edit `.env` to customize bot behavior:

| Variable | Description | Default |
|----------|-------------|---------|
| `PRIVATE_KEY` | Your wallet private key (required) | - |
| `ORDER_SIZE_USD` | USD amount per order | 10.0 |
| `SPREAD_OFFSET` | Price offset from best bid/ask | 0.01 |
| `CHECK_INTERVAL_SECONDS` | How often to check for new markets | 60 |
| `ORDER_PLACEMENT_MINUTES_BEFORE` | When to place orders before market start | 5 |
| `DASHBOARD_PORT` | Web dashboard port | 8000 |
| `LOG_LEVEL` | Logging verbosity (DEBUG, INFO, WARNING, ERROR) | INFO |

### Advanced: Proxy Wallets

For proxy wallets (email/Magic or browser-based), update `.env`:

```env
SIGNATURE_TYPE=POLY_PROXY  # or POLY_GNOSIS_SAFE
FUNDER_ADDRESS=0x...  # Your funder address
```

## Dashboard

The web dashboard provides real-time monitoring:

- **Status Bar**: Bot status, USDC balance, active markets count, pending orders
- **Markets Table**: Upcoming markets with countdowns and current prices
- **Open Orders**: Currently active limit orders
- **Recent Orders**: Order history with fill status
- **Logs**: Real-time bot activity logs

Access at: `http://localhost:8000`

## Project Structure

```
limitorderbot-claude/
├── main.py                 # Entry point
├── bot.py                  # Main bot logic
├── config.py              # Configuration management
├── logger.py              # Logging setup
├── models.py              # Data models (Pydantic)
├── market_discovery.py    # Gamma API integration
├── order_manager.py       # CLOB client & order management
├── dashboard.py           # FastAPI dashboard
├── templates/
│   └── dashboard.html     # Dashboard UI
├── requirements.txt       # Python dependencies
├── .env.example          # Example configuration
└── README.md             # This file
```

## Safety Features

- **Balance Checks**: Verifies sufficient USDC before placing orders
- **Order Validation**: Ensures prices are within valid range [0.01, 0.99]
- **Error Recovery**: Graceful handling of API failures and network issues
- **Automatic Cleanup**: Cancels stale orders to prevent capital lock-up
- **Rate Limiting**: Delays between orders to respect API limits

## Troubleshooting

### "Insufficient balance" error
- Ensure your wallet has enough USDC (at least 4 × ORDER_SIZE_USD)
- Check balance in dashboard

### No markets found
- BTC 15m markets may not be available at all times
- Bot will continue checking every 60 seconds
- Check Polymarket.com for active markets

### Orders not filling
- Market may have low liquidity
- Try adjusting `SPREAD_OFFSET` to be more competitive
- Orders will be auto-cancelled after market ends

### Private key errors
- Ensure PRIVATE_KEY in .env is correct (no 0x prefix needed if using raw hex)
- Check that the wallet has been used on Polymarket before

## Development

### Running Tests
```bash
# Add tests when ready
pytest tests/
```

### Modifying Strategy
- Edit `order_manager.py` to change order pricing logic
- Edit `bot.py` to change timing or market filtering
- Edit `config.py` to add new configuration options

## Security Notes

- **Never commit `.env` file** - it contains your private key
- Store private keys securely (consider using hardware wallets for production)
- Start with small ORDER_SIZE_USD amounts for testing
- Monitor the bot regularly, especially in the first few hours

## API References

- [Polymarket CLOB API](https://docs.polymarket.com)
- [py-clob-client Documentation](https://github.com/Polymarket/py-clob-client)
- [Gamma API](https://gamma-api.polymarket.com)

## License

MIT License - See LICENSE file for details

## Disclaimer

This bot is for educational purposes. Use at your own risk. Trading involves financial risk. Always test with small amounts first.

## Support

For issues or questions:
1. Check this README thoroughly
2. Review logs in `bot.log`
3. Open an issue on GitHub

---

Built with ❤️ for the Polymarket community
