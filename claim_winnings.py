"""Claim winnings from resolved markets."""
from order_manager import OrderManager
from config import Config
from logger import logger

print('='*60)
print('Claiming Winnings from Resolved Markets')
print('='*60)

# Initialize OrderManager
om = OrderManager(Config.PRIVATE_KEY)
print(f'Wallet: {om.address}\n')

# Get all resolved positions
try:
    print('Fetching resolved positions...')

    # The client doesn't have a direct "get resolved positions" method
    # We need to use the redeem_winnings method which handles this

    # Try to call the settlements endpoint to get claimable positions
    # This is typically done through the CTF contract

    print('\nAttempting to redeem all winnings...')

    # Use py-clob-client's built-in method if available
    if hasattr(om.client, 'redeem_winnings'):
        result = om.client.redeem_winnings()
        print(f'Redemption result: {result}')
    else:
        print('ERROR: redeem_winnings method not available in client')
        print('\nAlternative: You can redeem manually on polymarket.com')
        print('1. Go to https://polymarket.com/portfolio')
        print('2. Click on "Claimable" tab')
        print('3. Click "Claim All" button')

except Exception as e:
    logger.error(f'Error claiming winnings: {e}', exc_info=True)
    print(f'\nERROR: {e}')
    print('\nPlease redeem manually on Polymarket website:')
    print('1. Go to https://polymarket.com/portfolio')
    print('2. Click on "Claimable" tab')
    print('3. Click "Claim All"')

print('\n' + '='*60)
