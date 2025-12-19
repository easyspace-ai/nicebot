# Polymarket Position Redemption Guide

## Overview

After a Polymarket market resolves, you need to redeem your winning conditional tokens (CTF) to receive your USDC collateral back.

## Automated Redemption System

The bot now includes automated redemption functionality that will be integrated in future updates.

### Components Created

1. **market_tracker.py** - Tracks markets and their condition IDs
2. **auto_redeem.py** - Automated redemption service
3. **redeem_ctf_positions.py** - Manual redemption with market discovery
4. **redeem_now.py** - Quick redemption using token IDs

### How It Works

1. When bot places orders, it saves the market's `condition_id`
2. After market resolution, bot checks for unredeemed positions
3. Automatically calls `redeemPositions()` on CTF contract
4. Burns your CTF tokens and returns USDC to your wallet

## Manual Redemption (Current Situation)

For your current CTF tokens, you need the **condition ID** from the resolved market.

### Steps to Find Condition ID

1. Go to PolygonScan transaction (the one you provided):
   https://polygonscan.com/tx/0x01d760f7e2f279e959a70c83eafcd398619f4bbfcbc21e1dcad7b9d9f612730e

2. Click on "Event Logs" tab

3. Find the `ConditionResolution` or `PayoutRedemption` event (if available)
   - OR check the original market on Polymarket for the condition ID
   - OR extract it from the token ID mathematically

### Once You Have the Condition ID

Create a file `redeem_manual.py`:

```python
from auto_redeem import AutoRedeemer
from config import Config

# Your condition ID (32-byte hex string starting with 0x)
CONDITION_ID = "0x..."  # <-- PUT YOUR CONDITION ID HERE

redeemer = AutoRedeemer(Config.PRIVATE_KEY)
success = redeemer.redeem_market(
    market_slug="btc-updown-15m-1766141100",  # Or whatever the market was
    condition_id=CONDITION_ID,
    auto_confirm=True
)

if success:
    print("✓ Redemption successful!")
else:
    print("✗ Redemption failed - check logs")
```

Then run:
```bash
py redeem_manual.py
```

## Technical Details

### CTF Contract
- Address: `0x4D97DCd97eC945f40cF65F87097ACe5EA0476045`
- Function: `redeemPositions()`

### Parameters
- **collateralToken**: `0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174` (USDC.e)
- **parentCollectionId**: `0x0000...0000` (null for Polymarket)
- **conditionId**: The market's unique condition identifier (bytes32)
- **indexSets**: `[1, 2]` for binary markets (both outcomes)

### Token ID from Transaction
Token ID: `31422594496434086303292161195291941783825536358298906529346497914521519303924`

This is derived from: `keccak256(abi.encode(collectionId, conditionId, indexSet))`

## Future Integration

The bot will be updated to:
1. ✅ Track all markets with condition IDs (market_tracker.py)
2. ✅ Monitor for market resolutions
3. ✅ Automatically redeem winning positions (auto_redeem.py)
4. ✅ Display redemption status in dashboard
5. ⏳ Run periodic redemption checks (every hour)

## Troubleshooting

### "No claimable positions" on Polymarket Website
- This is expected for EOA wallets
- You have CTF tokens in your wallet but Polymarket UI doesn't show them
- Use the scripts above to redeem programmatically

### "Transaction reverted"
- Market may not be resolved yet
- Condition ID might be incorrect
- You may have already redeemed these positions

### "No balance"
Check your CTF token balance:
```bash
py check_positions.py
```

## Questions?

If you're stuck, provide:
1. Transaction hash of when you received CTF tokens
2. Market slug or question
3. Any error messages from redemption attempts
