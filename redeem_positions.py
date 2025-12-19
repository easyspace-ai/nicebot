"""Redeem all redeemable positions using Polymarket Positions API."""
from web3 import Web3
from config import Config
import requests

print('='*60)
print('Polymarket Position Redemption')
print('='*60)

# Initialize Web3
RPC_URL = Config.RPC_URL
w3 = Web3(Web3.HTTPProvider(RPC_URL))

if not w3.is_connected():
    print("ERROR: Cannot connect to Polygon")
    exit(1)

# Get wallet
private_key = Config.PRIVATE_KEY
account = w3.eth.account.from_key(private_key)
wallet = account.address

print(f'Wallet: {wallet}\n')

# Fetch positions from Polymarket API
print('Fetching positions from Polymarket API...')
api_url = f"https://data-api.polymarket.com/positions?user={wallet}"
response = requests.get(api_url)

if response.status_code != 200:
    print(f'ERROR: Failed to fetch positions (status {response.status_code})')
    exit(1)

positions = response.json()

if not positions:
    print('No positions found.')
    exit(0)

print(f'Found {len(positions)} position(s)\n')

# Filter redeemable positions
redeemable = [p for p in positions if p.get('redeemable', False)]

if not redeemable:
    print('No redeemable positions found.')
    print('All positions have been redeemed or markets are not yet resolved.')
    exit(0)

print(f'Found {len(redeemable)} redeemable position(s):\n')

# Group by condition ID
by_condition = {}
for pos in redeemable:
    cid = pos['conditionId']
    if cid not in by_condition:
        by_condition[cid] = []
    by_condition[cid].append(pos)

# Display summary
total_value = 0
for cid, positions in by_condition.items():
    market_title = positions[0]['title']
    market_slug = positions[0]['slug']
    market_value = sum(p['currentValue'] for p in positions)
    total_value += market_value

    print(f'Market: {market_title}')
    print(f'  Slug: {market_slug}')
    print(f'  Condition ID: {cid}')
    print(f'  Positions: {len(positions)}')
    print(f'  Total Value: ${market_value:.2f}')
    for p in positions:
        print(f'    - {p["outcome"]}: {p["size"]} shares @ ${p["curPrice"]:.2f} = ${p["currentValue"]:.2f}')
    print()

print(f'TOTAL REDEEMABLE VALUE: ${total_value:.2f}\n')

# Ask for confirmation
response = input(f'Redeem all positions? This will burn CTF tokens and return ${total_value:.2f} USDC to your wallet. (yes/no): ').strip().lower()

if response != 'yes':
    print('Redemption cancelled.')
    exit(0)

# Contract setup
CTF_ADDRESS = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"
USDC_ADDRESS = "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"

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

# Redeem each market
print('\n' + '='*60)
print('Starting Redemption')
print('='*60 + '\n')

success_count = 0
fail_count = 0

for cid, positions in by_condition.items():
    market_title = positions[0]['title']
    print(f'Redeeming: {market_title}')
    print(f'  Condition ID: {cid}')

    # Prepare redemption parameters
    collateral_token = Web3.to_checksum_address(USDC_ADDRESS)
    parent_collection_id = b'\x00' * 32  # Null for Polymarket
    condition_id_bytes = bytes.fromhex(cid[2:])  # Remove '0x'
    index_sets = [1, 2]  # Binary market: both outcomes

    try:
        # Build transaction
        nonce = w3.eth.get_transaction_count(wallet)
        gas_price = w3.eth.gas_price

        redeem_txn = ctf.functions.redeemPositions(
            collateral_token,
            parent_collection_id,
            condition_id_bytes,
            index_sets
        ).build_transaction({
            'from': wallet,
            'nonce': nonce,
            'gas': 300000,
            'gasPrice': gas_price,
            'chainId': 137
        })

        # Estimate gas
        try:
            estimated_gas = w3.eth.estimate_gas(redeem_txn)
            redeem_txn['gas'] = int(estimated_gas * 1.2)
            print(f'  Estimated Gas: {estimated_gas}')
        except Exception as e:
            print(f'  Could not estimate gas: {e}')
            print(f'  Using default: 300000')

        # Sign and send
        print('  Signing and sending transaction...')
        signed_txn = w3.eth.account.sign_transaction(redeem_txn, private_key)
        tx_hash = w3.eth.send_raw_transaction(signed_txn.raw_transaction)

        print(f'  [OK] Transaction sent!')
        print(f'  TX Hash: {tx_hash.hex()}')
        print(f'  PolygonScan: https://polygonscan.com/tx/{tx_hash.hex()}')

        # Wait for confirmation
        print('  Waiting for confirmation...')
        receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=120)

        if receipt.status == 1:
            print(f'  [SUCCESS] Redeemed!')
            print(f'  Gas Used: {receipt.gasUsed}')
            success_count += 1
        else:
            print(f'  [FAILED] Transaction reverted')
            fail_count += 1

    except Exception as e:
        print(f'  [ERROR] {e}')
        fail_count += 1

    print()

# Summary
print('='*60)
print('Redemption Complete')
print('='*60)
print(f'Successful: {success_count}/{len(by_condition)}')
print(f'Failed: {fail_count}/{len(by_condition)}')

if success_count > 0:
    print(f'\n${total_value:.2f} USDC has been returned to your wallet!')
    print('Check your balance with: py check_all_usdc.py')

print('='*60)
