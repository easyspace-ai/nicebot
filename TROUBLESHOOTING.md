# Troubleshooting Guide

Common issues and solutions for the Polymarket Limit Order Bot.

## Table of Contents
1. [Configuration Issues](#configuration-issues)
2. [Connection Problems](#connection-problems)
3. [Order Placement Issues](#order-placement-issues)
4. [Dashboard Issues](#dashboard-issues)
5. [Performance Issues](#performance-issues)
6. [Market Discovery Issues](#market-discovery-issues)

---

## Configuration Issues

### Error: "PRIVATE_KEY is required in .env file"

**Cause:** `.env` file missing or PRIVATE_KEY not set

**Solution:**
```bash
# 1. Check if .env exists
ls -la .env

# 2. If missing, copy from example
cp .env.example .env

# 3. Edit .env and add your private key
# Remove 0x prefix if present
PRIVATE_KEY=abc123...  # NOT 0xabc123...
```

---

### Error: "ORDER_SIZE_USD must be positive"

**Cause:** Invalid value in .env

**Solution:**
```env
# In .env, ensure positive number
ORDER_SIZE_USD=10.0  # ✓ Correct
ORDER_SIZE_USD=-10   # ✗ Wrong
ORDER_SIZE_USD=0     # ✗ Wrong
```

---

### Error: "Invalid SIGNATURE_TYPE"

**Cause:** Unsupported signature type

**Solution:**
```env
# Valid options:
SIGNATURE_TYPE=EOA              # ✓ Standard wallet
SIGNATURE_TYPE=POLY_PROXY       # ✓ Proxy wallet
SIGNATURE_TYPE=POLY_GNOSIS_SAFE # ✓ Gnosis Safe

# Invalid:
SIGNATURE_TYPE=OTHER            # ✗
```

For proxy wallets, also set:
```env
FUNDER_ADDRESS=0x...
```

---

## Connection Problems

### Error: "Failed to initialize CLOB client"

**Symptoms:**
```
ERROR - Failed to initialize CLOB client: ...
```

**Diagnosis:**
```bash
python test_connection.py
```

**Solutions:**

#### Solution 1: Check Private Key
```bash
# Ensure private key is 64 hex characters (without 0x)
# Should be 64 characters long
echo $PRIVATE_KEY | wc -c  # Should output 64
```

#### Solution 2: Check Internet Connection
```bash
# Test connectivity
curl https://clob.polymarket.com
curl https://gamma-api.polymarket.com
```

#### Solution 3: Check Polygon RPC
```env
# If using custom RPC, ensure it's working
# Default uses Polygon mainnet
CHAIN_ID=137
```

---

### Error: "Could not get orderbook"

**Cause:** CLOB API temporarily unavailable

**Solution:**
- This is usually transient
- Bot will retry on next iteration
- Check Polymarket status: https://polymarket.com

---

### Error: "Gamma API test failed"

**Cause:** Network issues or API changes

**Solutions:**

1. Check internet connection
2. Verify API endpoint:
```bash
curl https://gamma-api.polymarket.com/events
```

3. Check for API maintenance announcements

---

## Order Placement Issues

### Error: "Insufficient balance"

**Symptoms:**
```
ERROR - Insufficient balance: $5.00 < $40.00
```

**Solution:**

1. Check actual balance:
```bash
# Visit Polygonscan
# https://polygonscan.com/address/YOUR_ADDRESS
```

2. Add USDC to your wallet:
   - Bridge from Ethereum
   - Buy on exchange and withdraw to Polygon
   - Use faucet for testing

3. Or reduce order size:
```env
# In .env
ORDER_SIZE_USD=5.0  # Reduce from 10.0
```

Remember: 4 orders × ORDER_SIZE_USD = total required

---

### Issue: "Orders placed but not filling"

**Causes:**
- Spread too wide
- Low market liquidity
- Unfavorable pricing

**Solutions:**

1. Check your order prices in dashboard
2. Compare to market prices on Polymarket.com
3. Reduce spread offset for more competitive pricing:
```env
# In .env
SPREAD_OFFSET=0.005  # More aggressive (was 0.01)
```

4. Monitor first few markets to calibrate strategy

---

### Error: "No order ID in response"

**Symptoms:**
```
ERROR - No order ID in response: {...}
```

**Causes:**
- Order validation failed
- Price out of bounds
- Insufficient allowance

**Solutions:**

1. Check logs for detailed error message
2. Verify allowances are set:
```bash
# Run test to verify client initialization
python test_connection.py
```

3. Restart bot to re-initialize allowances:
```bash
python main.py
```

---

### Issue: "Orders cancelled immediately"

**Cause:** Market may have started or ended

**Solution:**
- Check market timing in dashboard
- Verify CHECK_INTERVAL_SECONDS isn't too large:
```env
# In .env - reduce for more precise timing
CHECK_INTERVAL_SECONDS=30  # Check more frequently
```

---

## Dashboard Issues

### Error: "Dashboard not loading (localhost:8000)"

**Solutions:**

1. Check if bot is running:
```bash
# Should see: "Starting dashboard on 0.0.0.0:8000"
```

2. Check port is not in use:
```bash
# Windows
netstat -ano | findstr :8000

# Mac/Linux
lsof -i :8000
```

3. Change port if needed:
```env
# In .env
DASHBOARD_PORT=8001
```

4. Check firewall:
   - Allow port 8000
   - Or use localhost only

---

### Issue: "Dashboard shows $0.00 balance"

**Causes:**
- Balance not yet fetched
- API error
- No USDC in wallet

**Solutions:**

1. Wait for next update (5 seconds)
2. Check bot logs for errors
3. Verify wallet has USDC on Polygonscan
4. Restart bot

---

### Issue: "No markets showing in dashboard"

**Causes:**
- No BTC 15m markets currently available
- Market discovery failed
- Timing issue

**Solutions:**

1. Check bot logs:
```bash
tail -f bot.log | grep "Discovered"
```

2. Verify markets exist on Polymarket.com:
   - Search for "Bitcoin Up or Down"
   - Check if any are 15-minute markets

3. Be patient - markets may not always be available

4. Check Gamma API manually:
```bash
curl https://gamma-api.polymarket.com/events | grep -i bitcoin
```

---

### Issue: "Dashboard not auto-refreshing"

**Solution:**
- Check browser console (F12) for JavaScript errors
- Try hard refresh: Ctrl+F5 (Windows) or Cmd+Shift+R (Mac)
- Clear browser cache
- Try different browser

---

## Performance Issues

### Issue: "Bot using too much memory"

**Cause:** Old markets not being cleaned up

**Solution:**
Already handled in code, but if issue persists:

1. Restart bot daily
2. Monitor with:
```bash
# Linux/Mac
ps aux | grep python

# Windows Task Manager
```

3. Check for market buildup:
```python
# In bot logs, look for cleanup messages
"Cleaning up old market: ..."
```

---

### Issue: "Slow order placement"

**Cause:** API latency or rate limiting

**Solutions:**

1. Check internet speed
2. Verify not hitting rate limits (delays between orders)
3. Consider moving to server with better connectivity
4. Check bot logs for slow responses

---

### Issue: "Bot stopped responding"

**Symptoms:**
- Dashboard not updating
- No new log entries
- Last check time frozen

**Solutions:**

1. Check if process still running:
```bash
# Windows
tasklist | findstr python

# Linux/Mac
ps aux | grep python
```

2. Check for errors in bot.log:
```bash
tail -100 bot.log
```

3. Restart bot:
```bash
# Ctrl+C to stop
# Then:
python main.py
```

4. Check system resources (CPU, memory, disk)

---

## Market Discovery Issues

### Issue: "Markets found but wrong type"

**Symptoms:**
```
Found 5 markets but none are BTC 15m
```

**Cause:** API returning non-BTC or different duration markets

**Solution:**
- This is expected - bot filters correctly
- Only BTC 15-minute markets will be tracked
- Check logs for "Tracking new market: btc-updown-15m-..."

---

### Issue: "Missing market timestamp"

**Symptoms:**
```
Could not extract timestamps for market: ...
```

**Cause:** API response format changed or incomplete data

**Solutions:**

1. Check if Gamma API changed format
2. Review market data structure:
```python
# Add debug logging to market_discovery.py
logger.debug(f"Market data: {market_data}")
```

3. Report issue with specific market slug

---

### Issue: "Market detected too late"

**Symptoms:**
- Market starts before orders placed
- Countdown shows negative time

**Cause:** CHECK_INTERVAL_SECONDS too large

**Solution:**
```env
# In .env - reduce check interval
CHECK_INTERVAL_SECONDS=30  # Check every 30s instead of 60s
```

Trade-off: More frequent checks = more API calls

---

## General Debugging Steps

### Step 1: Check Configuration
```bash
python main.py --check-config
```

### Step 2: Test Connections
```bash
python test_connection.py
```

### Step 3: Review Logs
```bash
# View full log
cat bot.log

# Follow live
tail -f bot.log

# Search for errors
grep ERROR bot.log

# Search for specific market
grep "btc-updown-15m" bot.log
```

### Step 4: Check Dashboard
```
http://localhost:8000
```
- Review status panel
- Check error count
- View last error message
- Examine recent orders

### Step 5: Verify Wallet
- Check balance on Polygonscan
- Verify transactions
- Check USDC allowances

### Step 6: Test API Manually
```bash
# Gamma API
curl https://gamma-api.polymarket.com/events

# CLOB API (requires auth)
# Use test_connection.py instead
```

---

## Getting Help

If you still have issues:

1. **Gather Information:**
   - Error messages from bot.log
   - Configuration (without private key!)
   - Steps to reproduce
   - Expected vs actual behavior

2. **Check Documentation:**
   - README.md
   - ARCHITECTURE.md
   - This guide

3. **Search Logs:**
   ```bash
   grep -A 5 -B 5 "ERROR" bot.log
   ```

4. **Describe Your Setup:**
   - Operating system
   - Python version
   - Installation method
   - When did it start failing?

---

## Common Log Messages Explained

### Normal Messages
```
INFO - Discovering BTC 15-minute markets...
INFO - Found 2 upcoming/active markets
INFO - Sleeping for 60 seconds...
```
✓ Bot working normally

```
INFO - Tracking new market: btc-updown-15m-...
```
✓ New market discovered

```
INFO - Order placed successfully: ...
```
✓ Order submitted successfully

```
INFO - Order ... filled completely
```
✓ Order executed successfully

### Warning Messages
```
WARNING - Could not get orderbook for ...
```
⚠ Transient API issue, will retry

```
WARNING - Could not set USDC allowance: ...
```
⚠ May already be set, check if orders work

### Error Messages
```
ERROR - Insufficient balance: ...
```
❌ Add USDC to wallet

```
ERROR - Error placing order: ...
```
❌ Check specific error details in log

```
ERROR - No order ID in response: ...
```
❌ Order submission failed, check allowances

---

## Quick Reference

| Problem | Quick Fix |
|---------|-----------|
| Can't start bot | Run `python main.py --check-config` |
| No balance showing | Wait 5 seconds or check Polygonscan |
| No markets found | Normal - markets not always available |
| Orders not filling | Reduce SPREAD_OFFSET |
| Dashboard not loading | Check port 8000 not in use |
| Connection errors | Run `python test_connection.py` |

---

## Prevention Tips

1. **Start Small:** Test with ORDER_SIZE_USD=5 first
2. **Monitor Initially:** Watch first few market cycles
3. **Check Balance:** Ensure sufficient USDC
4. **Review Logs:** Check bot.log regularly
5. **Keep Updated:** Update dependencies periodically
6. **Backup Config:** Save working .env configuration

---

**Remember:** Most issues are configuration-related. Always start with `test_connection.py` when debugging!
