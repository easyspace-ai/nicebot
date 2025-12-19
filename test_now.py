"""Quick test of order placement."""
import sys
from order_manager import OrderManager
from market_discovery import MarketDiscovery
from config import Config
from logger import logger

print('\n' + '='*60)
print('Test Order Placement')
print('='*60)

om = OrderManager(Config.PRIVATE_KEY)
print(f'Wallet: {om.address}')

disc = MarketDiscovery()
markets = disc.discover_btc_15m_markets()
market = markets[0] if markets else None
print(f'Market: {market.market_slug if market else "None"}')

if market:
    orders = om.place_simple_test_orders(market=market, price=0.49, size=10.0)
    print(f'\nResults:')
    for o in orders:
        status_msg = o.order_id if o.status.value != 'FAILED' else o.error_message
        print(f'  {o.side.value} {o.outcome}: {o.status.value} - {status_msg}')

    success = sum(1 for o in orders if o.status.value == 'PLACED')
    print(f'\nSuccess: {success}/{len(orders)} orders placed')
    sys.exit(0 if success > 0 else 1)
else:
    print('No markets found')
    sys.exit(1)
