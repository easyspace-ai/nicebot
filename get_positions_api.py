"""Get positions using Polymarket CLOB API."""
from order_manager import OrderManager
from config import Config
from logger import logger
import json

print('='*60)
print('Fetching Positions via Polymarket API')
print('='*60)

om = OrderManager(Config.PRIVATE_KEY)
print(f'Wallet: {om.address}\n')

try:
    # Try different API methods to get positions
    print('Method 1: Checking balances...')
    try:
        if hasattr(om.client, 'get_balance_allowance'):
            balance_info = om.client.get_balance_allowance()
            print(f'Balance info: {json.dumps(balance_info, indent=2)}\n')
    except Exception as e:
        print(f'Error: {e}\n')

    print('Method 2: Checking positions...')
    try:
        # Some CLOB clients have a positions endpoint
        import requests
        url = f"https://clob.polymarket.com/positions"
        headers = {
            'Authorization': f'Bearer {om.client.creds.api_key}' if hasattr(om.client, 'creds') and om.client.creds else ''
        }
        params = {'user': om.address}

        response = requests.get(url, headers=headers, params=params)
        print(f'Status: {response.status_code}')
        print(f'Response: {json.dumps(response.json(), indent=2)}\n')

    except Exception as e:
        print(f'Error: {e}\n')

    print('Method 3: Checking via Gamma Markets API...')
    try:
        import requests
        # Gamma Markets API can show positions
        url = f"https://gamma-api.polymarket.com/positions"
        params = {'user': om.address}

        response = requests.get(url, params=params)
        print(f'Status: {response.status_code}')

        if response.status_code == 200:
            positions = response.json()
            print(f'Response: {json.dumps(positions, indent=2)}')

            if isinstance(positions, list) and len(positions) > 0:
                print(f'\nFound {len(positions)} position(s)!')
                for pos in positions:
                    print(f'\nPosition:')
                    print(f'  Market: {pos.get("market", "N/A")}')
                    print(f'  Outcome: {pos.get("outcome", "N/A")}')
                    print(f'  Size: {pos.get("size", 0)}')
                    print(f'  Value: ${pos.get("value", 0)}')
            else:
                print('\nNo positions found via Gamma API')
        else:
            print(f'Error: {response.text}')

    except Exception as e:
        print(f'Error: {e}\n')

except Exception as e:
    logger.error(f'Error: {e}', exc_info=True)

print('='*60)
print('\nIf you can see CTF tokens in your wallet/portfolio,')
print('please provide the token ID so we can redeem it directly.')
print('='*60)
