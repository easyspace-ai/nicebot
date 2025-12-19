"""Set all required allowances for Polymarket trading."""
from web3 import Web3
from config import Config

print('Setting ALL Polymarket Allowances')
print('='*60)

# Polygon RPC endpoint
RPC_URL = "https://polygon-rpc.com"

# Contract addresses on Polygon
USDC_ADDRESS = "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"
CTF_ADDRESS = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"  # Conditional Tokens

# Spender addresses (all three exchanges)
SPENDERS = [
    ("0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E", "CTF Exchange"),
    ("0xC5d563A36AE78145C45a50134d48A1215220f80a", "Neg Risk CTF Exchange"),
    ("0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296", "Neg Risk Adapter")
]

# ERC20 approve ABI
ERC20_ABI = [{
    "constant": False,
    "inputs": [
        {"name": "_spender", "type": "address"},
        {"name": "_value", "type": "uint256"}
    ],
    "name": "approve",
    "outputs": [{"name": "", "type": "bool"}],
    "type": "function"
}]

# ERC1155 setApprovalForAll ABI
ERC1155_ABI = [{
    "constant": False,
    "inputs": [
        {"name": "operator", "type": "address"},
        {"name": "approved", "type": "bool"}
    ],
    "name": "setApprovalForAll",
    "outputs": [],
    "type": "function"
}, {
    "constant": True,
    "inputs": [
        {"name": "account", "type": "address"},
        {"name": "operator", "type": "address"}
    ],
    "name": "isApprovedForAll",
    "outputs": [{"name": "", "type": "bool"}],
    "type": "function"
}]

# Initialize Web3
w3 = Web3(Web3.HTTPProvider(RPC_URL))

if not w3.is_connected():
    print("ERROR: Could not connect to Polygon RPC")
    exit(1)

print(f"Connected to Polygon (Chain ID: {w3.eth.chain_id})")

# Get private key and account
private_key = Config.PRIVATE_KEY
account = w3.eth.account.from_key(private_key)
wallet_address = account.address

print(f"Wallet: {wallet_address}\n")

# Create contract instances
usdc_contract = w3.eth.contract(
    address=Web3.to_checksum_address(USDC_ADDRESS),
    abi=ERC20_ABI
)

ctf_contract = w3.eth.contract(
    address=Web3.to_checksum_address(CTF_ADDRESS),
    abi=ERC1155_ABI
)

# Track transactions
txs = []

# Approve amount (1 million USDC)
approve_amount = 1_000_000 * 1_000_000

for spender_address, spender_name in SPENDERS:
    print(f"Processing {spender_name}...")
    print(f"  Address: {spender_address}")

    spender_checksum = Web3.to_checksum_address(spender_address)

    # 1. Check and approve USDC (ERC20)
    print(f"  [1/2] Approving USDC...")
    try:
        # Build USDC approve transaction
        nonce = w3.eth.get_transaction_count(wallet_address)
        gas_price = w3.eth.gas_price

        approve_txn = usdc_contract.functions.approve(
            spender_checksum,
            approve_amount
        ).build_transaction({
            'from': wallet_address,
            'nonce': nonce,
            'gas': 100000,
            'gasPrice': gas_price,
            'chainId': 137
        })

        # Sign and send
        signed_txn = w3.eth.account.sign_transaction(approve_txn, private_key)
        tx_hash = w3.eth.send_raw_transaction(signed_txn.raw_transaction)

        print(f"        TX: {tx_hash.hex()}")

        # Wait for confirmation
        receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=120)

        if receipt.status == 1:
            print(f"        SUCCESS (Gas: {receipt.gasUsed})")
            txs.append((spender_name, "USDC", tx_hash.hex()))
        else:
            print(f"        FAILED")

    except Exception as e:
        print(f"        ERROR: {e}")

    # 2. Check and approve CTF (ERC1155)
    print(f"  [2/2] Approving Conditional Tokens...")
    try:
        # Check if already approved
        is_approved = ctf_contract.functions.isApprovedForAll(
            wallet_address,
            spender_checksum
        ).call()

        if is_approved:
            print(f"        Already approved, skipping")
        else:
            # Build setApprovalForAll transaction
            nonce = w3.eth.get_transaction_count(wallet_address)
            gas_price = w3.eth.gas_price

            approval_txn = ctf_contract.functions.setApprovalForAll(
                spender_checksum,
                True
            ).build_transaction({
                'from': wallet_address,
                'nonce': nonce,
                'gas': 100000,
                'gasPrice': gas_price,
                'chainId': 137
            })

            # Sign and send
            signed_txn = w3.eth.account.sign_transaction(approval_txn, private_key)
            tx_hash = w3.eth.send_raw_transaction(signed_txn.raw_transaction)

            print(f"        TX: {tx_hash.hex()}")

            # Wait for confirmation
            receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=120)

            if receipt.status == 1:
                print(f"        SUCCESS (Gas: {receipt.gasUsed})")
                txs.append((spender_name, "CTF", tx_hash.hex()))
            else:
                print(f"        FAILED")

    except Exception as e:
        print(f"        ERROR: {e}")

    print()

# Summary
print('='*60)
print('SUMMARY')
print('='*60)
print(f'Completed {len(txs)} transactions:')
for spender, token, tx in txs:
    print(f'  - {spender} ({token}): {tx}')

print('\nAll allowances have been set!')
print('You can now trade on Polymarket with the bot.')
