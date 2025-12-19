"""Redeem positions using token IDs from PolygonScan."""
from web3 import Web3
from config import Config

print('='*60)
print('Redeem Positions from Token IDs')
print('='*60)

RPC_URL = Config.RPC_URL
w3 = Web3(Web3.HTTPProvider(RPC_URL))

# Get wallet
private_key = Config.PRIVATE_KEY
account = w3.eth.account.from_key(private_key)
wallet = account.address

print(f'Wallet: {wallet}\n')

# Token IDs from the transactions (from PolygonScan)
# Transaction: 0x01d760f7e2f279e959a70c83eafcd398619f4bbfcbc21e1dcad7b9d9f612730e
TOKEN_ID_1 = 31422594496434086303292161195291941783825536358298906529346497914521519303924

# Transaction: 0x90a42ead6c... (second one - you'll need to provide full hash)
# For now, let me check what the actual condition ID should be

# The token ID format for Polymarket CTF is:
# positionId = keccak256(abi.encode(collectionId, conditionId, indexSet))

# We need to work backwards or get the condition ID from the market
# The easiest way is to check recent BTC markets

from market_discovery import MarketDiscovery

print('Searching for recent BTC markets to find condition ID...\n')

disc = MarketDiscovery()
markets = disc.discover_btc_15m_markets()

# Look for markets around the time of your orders (timestamp 1766141100 or 1766142000)
TARGET_TIMESTAMPS = [1766141100, 1766142000]

found_markets = []
for market in markets:
    # Markets have end_date_iso which we can parse
    if hasattr(market, 'end_time'):
        if market.end_time in TARGET_TIMESTAMPS:
            found_markets.append(market)
            print(f'Found market: {market.market_slug}')
            print(f'  Condition ID: {market.condition_id}')
            print(f'  End time: {market.end_time}')
            print()

if not found_markets:
    print('Could not find the exact market in current API results.')
    print('The market likely resolved and was removed.')
    print()
    print('MANUAL REDEMPTION REQUIRED')
    print('-' * 60)
    print('Based on the PolygonScan transaction, you have token ID:')
    print(f'  {TOKEN_ID_1}')
    print()
    print('To redeem, we need the condition ID.')
    print('Please check the Polymarket website or provide the condition ID')
    print('from the original market data.')
    print()
    print('Once you have the condition ID, run:')
    print('  py auto_redeem.py')
else:
    # Try to redeem using the found market's condition ID
    print('Attempting redemption...\n')

    from auto_redeem import AutoRedeemer

    redeemer = AutoRedeemer(Config.PRIVATE_KEY)

    for market in found_markets:
        success = redeemer.redeem_market(
            market.market_slug,
            market.condition_id,
            auto_confirm=True
        )

        if success:
            print(f'✓ Successfully redeemed {market.market_slug}')
        else:
            print(f'✗ Failed to redeem {market.market_slug}')

print('='*60)
