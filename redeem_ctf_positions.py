"""Redeem CTF positions after market resolution."""
from web3 import Web3
from config import Config
from logger import logger

print('='*60)
print('Redeem CTF Positions')
print('='*60)

RPC_URL = "https://polygon-rpc.com"
w3 = Web3(Web3.HTTPProvider(RPC_URL))

if not w3.is_connected():
    print("ERROR: Cannot connect to Polygon")
    exit(1)

# Get wallet
private_key = Config.PRIVATE_KEY
account = w3.eth.account.from_key(private_key)
wallet = account.address

print(f'Wallet: {wallet}\n')

# Contract addresses
CTF_ADDRESS = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"
USDC_ADDRESS = "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"

# redeemPositions ABI
REDEEM_ABI = [{
    "constant": False,
    "inputs": [
        {"name": "collateralToken", "type": "address"},
        {"name": "parentCollectionId", "type": "bytes32"},
        {"name": "conditionId", "type": "bytes32"},
        {"name": "indexSets", "type": "uint256[]"}
    ],
    "name": "redeemPositions",
    "outputs": [],
    "type": "function"
}]

ctf = w3.eth.contract(
    address=Web3.to_checksum_address(CTF_ADDRESS),
    abi=REDEEM_ABI
)

# We need to get the conditionId from the market
# The market that just resolved was: btc-updown-15m-1766141100

# For BTC 15-minute markets, we need the condition ID
# This can be derived from the market data or gotten from the bot's tracking

print('To redeem your positions, we need:')
print('1. conditionId - from the resolved market')
print('2. indexSets - typically [1, 2] for binary markets\n')

# Let's try to get this from the order manager
from order_manager import OrderManager
from market_discovery import MarketDiscovery

try:
    om = OrderManager(Config.PRIVATE_KEY)
    disc = MarketDiscovery()

    print('Checking recent markets...\n')

    # Get all BTC markets
    markets = disc.discover_btc_15m_markets()

    # Look for the market that recently resolved (btc-updown-15m-1766141100)
    target_market_slug = "btc-updown-15m-1766141100"

    market = None
    for m in markets:
        if m.market_slug == target_market_slug:
            market = m
            break

    if market:
        print(f'Found market: {market.market_slug}')
        print(f'Condition ID: {market.condition_id}')
        print(f'Question: {market.question}')
        print()

        # Prepare redemption parameters
        collateral_token = Web3.to_checksum_address(USDC_ADDRESS)
        parent_collection_id = b'\x00' * 32  # Null bytes for Polymarket
        condition_id = bytes.fromhex(market.condition_id[2:])  # Remove '0x' prefix
        index_sets = [1, 2]  # Binary market: both outcomes

        print('Redemption parameters:')
        print(f'  Collateral: {collateral_token}')
        print(f'  Parent Collection: 0x{"00" * 32}')
        print(f'  Condition ID: {market.condition_id}')
        print(f'  Index Sets: {index_sets}')
        print()

        # Confirm before executing
        print('Ready to redeem positions!')
        print('This will burn your CTF tokens and return USDC.')
        print()

        response = input('Proceed with redemption? (yes/no): ')

        if response.lower() != 'yes':
            print('Redemption cancelled.')
            exit(0)

        print('\nBuilding redemption transaction...')

        # Get current nonce and gas price
        nonce = w3.eth.get_transaction_count(wallet)
        gas_price = w3.eth.gas_price

        # Build transaction
        redeem_txn = ctf.functions.redeemPositions(
            collateral_token,
            parent_collection_id,
            condition_id,
            index_sets
        ).build_transaction({
            'from': wallet,
            'nonce': nonce,
            'gas': 300000,  # Estimated gas
            'gasPrice': gas_price,
            'chainId': 137
        })

        print(f'  Nonce: {nonce}')
        print(f'  Gas Price: {w3.from_wei(gas_price, "gwei")} gwei')
        print(f'  Gas Limit: 300000')

        # Estimate actual gas
        try:
            estimated_gas = w3.eth.estimate_gas(redeem_txn)
            redeem_txn['gas'] = int(estimated_gas * 1.2)  # Add 20% buffer
            print(f'  Estimated Gas: {estimated_gas}')
        except Exception as e:
            print(f'  Could not estimate gas: {e}')

        print('\nSigning transaction...')
        signed_txn = w3.eth.account.sign_transaction(redeem_txn, private_key)

        print('Sending transaction...')
        tx_hash = w3.eth.send_raw_transaction(signed_txn.raw_transaction)

        print(f'\nTransaction sent!')
        print(f'TX Hash: {tx_hash.hex()}')
        print(f'PolygonScan: https://polygonscan.com/tx/{tx_hash.hex()}')

        print('\nWaiting for confirmation...')
        receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=120)

        if receipt.status == 1:
            print('\n' + '='*60)
            print('SUCCESS! Positions redeemed.')
            print('='*60)
            print(f'Block: {receipt.blockNumber}')
            print(f'Gas Used: {receipt.gasUsed}')
            print('\nYour USDC has been returned to your wallet!')
        else:
            print('\n' + '='*60)
            print('FAILED! Transaction reverted.')
            print('='*60)
            print(f'Check transaction: https://polygonscan.com/tx/{tx_hash.hex()}')
    else:
        print(f'ERROR: Could not find market {target_market_slug}')
        print('The market may have been removed from the API.')
        print('\nTo redeem manually:')
        print('1. Find the conditionId from the market data')
        print('2. Call redeemPositions with:')
        print('   - collateralToken: 0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174')
        print('   - parentCollectionId: 0x' + '00' * 32)
        print('   - conditionId: <from market>')
        print('   - indexSets: [1, 2]')

except Exception as e:
    logger.error(f'Error: {e}', exc_info=True)
    print(f'\nERROR: {e}')

print('\n' + '='*60)
