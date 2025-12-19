"""Update L2 balance allowance for Polymarket trading."""
from order_manager import OrderManager
from config import Config
from logger import logger

print('Updating L2 Balance/Allowance on Polymarket')
print('='*60)

# Initialize OrderManager
om = OrderManager(Config.PRIVATE_KEY)
print(f'Wallet: {om.address}\n')

# Try to update balance allowance for USDC (collateral)
print('Updating COLLATERAL (USDC) balance allowance...')
try:
    from py_clob_client.clob_types import BalanceAllowanceParams, AssetType

    # Update for USDC collateral
    params = BalanceAllowanceParams(asset_type=AssetType.COLLATERAL)
    result = om.client.update_balance_allowance(params)
    print(f'Result: {result}')
    print('SUCCESS!\n')
except Exception as e:
    print(f'ERROR: {e}\n')

# Check balance
print('Checking L2 balance...')
try:
    balance_info = om.client.get_balance_allowance()
    print(f'Balance info: {balance_info}')
except Exception as e:
    print(f'ERROR: {e}')

print('\n' + '='*60)
print('Done! You should now be able to place orders.')
