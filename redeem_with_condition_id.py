"""Simple redemption script - just provide the condition ID."""
from web3 import Web3
from config import Config

# ============================================================
# CONFIGURATION - UPDATE THIS WITH YOUR CONDITION ID
# ============================================================

# Get the condition ID from:
# 1. PolygonScan transaction event logs
# 2. Original market data
# 3. Polymarket API (if market still exists)

CONDITION_ID = input("Enter the condition ID (0x...): ").strip()

if not CONDITION_ID or not CONDITION_ID.startswith("0x") or len(CONDITION_ID) != 66:
    print("ERROR: Invalid condition ID format")
    print("Should be a 32-byte hex string starting with 0x (66 characters total)")
    exit(1)

# ============================================================
# REDEMPTION CODE
# ============================================================

print('='*60)
print('Redeeming CTF Positions')
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

print(f'Wallet: {wallet}')
print(f'Condition ID: {CONDITION_ID}\n')

# Contract addresses
CTF_ADDRESS = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"
USDC_ADDRESS = "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"

# ABI
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

# Redemption parameters
collateral_token = Web3.to_checksum_address(USDC_ADDRESS)
parent_collection_id = b'\x00' * 32
condition_id_bytes = bytes.fromhex(CONDITION_ID[2:])
index_sets = [1, 2]  # Binary market: redeem both Up and Down

print('Building redemption transaction...')

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

    print(f'  Nonce: {nonce}')
    print(f'  Gas Price: {w3.from_wei(gas_price, "gwei"):.2f} gwei')

    # Estimate gas
    try:
        estimated_gas = w3.eth.estimate_gas(redeem_txn)
        redeem_txn['gas'] = int(estimated_gas * 1.2)
        print(f'  Estimated Gas: {estimated_gas}')
        tx_cost_matic = w3.from_wei(redeem_txn['gas'] * gas_price, 'ether')
        print(f'  TX Cost: ~{tx_cost_matic:.6f} MATIC')
    except Exception as e:
        print(f'  Could not estimate gas: {e}')
        print(f'  Using default: 300000')

    print('\nSigning transaction...')
    signed_txn = w3.eth.account.sign_transaction(redeem_txn, private_key)

    print('Sending transaction...')
    tx_hash = w3.eth.send_raw_transaction(signed_txn.raw_transaction)

    print(f'\n[OK] Transaction sent!')
    print(f'TX Hash: {tx_hash.hex()}')
    print(f'PolygonScan: https://polygonscan.com/tx/{tx_hash.hex()}')

    print('\nWaiting for confirmation (max 2 minutes)...')
    receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=120)

    print('\n' + '='*60)
    if receipt.status == 1:
        print('SUCCESS! Positions redeemed.')
        print('='*60)
        print(f'Block: {receipt.blockNumber}')
        print(f'Gas Used: {receipt.gasUsed}')
        print('\nYour USDC has been returned to your wallet!')
        print('Check balance with: py check_all_usdc.py')
    else:
        print('FAILED! Transaction reverted.')
        print('='*60)
        print('Possible reasons:')
        print('- Market not yet resolved')
        print('- Wrong condition ID')
        print('- Already redeemed')
        print(f'\nCheck TX: https://polygonscan.com/tx/{tx_hash.hex()}')

except Exception as e:
    print(f'\nERROR: {e}')
    import traceback
    traceback.print_exc()

print('\n' + '='*60)
