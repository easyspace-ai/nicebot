"""Check open orders on Polymarket."""
from order_manager import OrderManager
from config import Config
from logger import logger

print('='*60)
print('Checking Open Orders')
print('='*60)

om = OrderManager(Config.PRIVATE_KEY)
print(f'Wallet: {om.address}\n')

try:
    # Get open orders from Polymarket
    print('Fetching open orders from Polymarket...\n')

    if hasattr(om.client, 'get_orders'):
        orders = om.client.get_orders()
        print(f'Raw response: {orders}\n')

        if isinstance(orders, list):
            if len(orders) == 0:
                print('No open orders found.')
            else:
                print(f'Found {len(orders)} open order(s):\n')
                for order in orders:
                    print(f'Order: {order}')
                    print()
        else:
            print(f'Unexpected response format: {type(orders)}')
    else:
        print('ERROR: get_orders method not available')
        print('Trying alternative method...')

        # Try to get orders for a specific market
        # You would need to provide the market ID

except Exception as e:
    logger.error(f'Error fetching orders: {e}', exc_info=True)
    print(f'ERROR: {e}')

print('='*60)
