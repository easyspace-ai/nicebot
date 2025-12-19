"""Set USDC allowance for Polymarket trading."""
from web3 import Web3
from config import Config

print('Setting USDC Allowance for Polymarket Trading')
print('='*60)

# Polygon RPC endpoint
RPC_URL = "https://polygon-rpc.com"

# Contract addresses on Polygon
USDC_ADDRESS = "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"  # USDC on Polygon
EXCHANGE_ADDRESS = "0xC5d563A36AE78145C45a50134d48A1215220f80a"  # Polymarket NegRisk CTF Exchange

# ERC20 approve function ABI
ERC20_ABI = [
    {
        "constant": False,
        "inputs": [
            {"name": "_spender", "type": "address"},
            {"name": "_value", "type": "uint256"}
        ],
        "name": "approve",
        "outputs": [{"name": "", "type": "bool"}],
        "type": "function"
    },
    {
        "constant": True,
        "inputs": [
            {"name": "_owner", "type": "address"},
            {"name": "_spender", "type": "address"}
        ],
        "name": "allowance",
        "outputs": [{"name": "", "type": "uint256"}],
        "type": "function"
    }
]

# Initialize Web3
w3 = Web3(Web3.HTTPProvider(RPC_URL))

if not w3.is_connected():
    print("ERROR: Could not connect to Polygon RPC")
    print("Please check your internet connection")
    exit(1)

print(f"Connected to Polygon (Chain ID: {w3.eth.chain_id})")

# Get private key and account
private_key = Config.PRIVATE_KEY
account = w3.eth.account.from_key(private_key)
wallet_address = account.address

print(f"Wallet: {wallet_address}")

# Create USDC contract instance
usdc_contract = w3.eth.contract(
    address=Web3.to_checksum_address(USDC_ADDRESS),
    abi=ERC20_ABI
)

# Check current allowance
print(f"\nChecking current USDC allowance...")
current_allowance = usdc_contract.functions.allowance(
    Web3.to_checksum_address(wallet_address),
    Web3.to_checksum_address(EXCHANGE_ADDRESS)
).call()

current_allowance_usdc = current_allowance / 1_000_000  # USDC has 6 decimals
print(f"Current allowance: ${current_allowance_usdc:,.2f} USDC")

if current_allowance_usdc >= 1000:
    print(f"\nAllowance is already set! You're good to go.")
    exit(0)

# Set allowance to 1 million USDC (unlimited for practical purposes)
approve_amount = 1_000_000 * 1_000_000  # 1M USDC with 6 decimals

print(f"\nSetting allowance to $1,000,000 USDC...")
print(f"  Spender: {EXCHANGE_ADDRESS}")
print(f"  Amount: {approve_amount}")

# Build transaction
nonce = w3.eth.get_transaction_count(wallet_address)
gas_price = w3.eth.gas_price

print(f"\nBuilding transaction...")
print(f"  Nonce: {nonce}")
print(f"  Gas Price: {w3.from_wei(gas_price, 'gwei')} gwei")

# Build approve transaction
approve_txn = usdc_contract.functions.approve(
    Web3.to_checksum_address(EXCHANGE_ADDRESS),
    approve_amount
).build_transaction({
    'from': wallet_address,
    'nonce': nonce,
    'gas': 100000,  # Typical gas limit for ERC20 approve
    'gasPrice': gas_price,
    'chainId': 137
})

# Estimate gas
try:
    estimated_gas = w3.eth.estimate_gas(approve_txn)
    approve_txn['gas'] = int(estimated_gas * 1.2)  # Add 20% buffer
    print(f"  Estimated Gas: {estimated_gas}")
    print(f"  Gas Limit (with buffer): {approve_txn['gas']}")

    # Calculate cost
    tx_cost_wei = approve_txn['gas'] * gas_price
    tx_cost_matic = w3.from_wei(tx_cost_wei, 'ether')
    print(f"  Transaction Cost: {tx_cost_matic:.6f} MATIC")
except Exception as e:
    print(f"  Could not estimate gas: {e}")
    print(f"  Using default gas limit: 100000")

# Sign transaction
print(f"\nSigning transaction...")
signed_txn = w3.eth.account.sign_transaction(approve_txn, private_key)

# Send transaction
print(f"Sending transaction...")
try:
    tx_hash = w3.eth.send_raw_transaction(signed_txn.raw_transaction)
    print(f"\nTransaction sent!")
    print(f"  TX Hash: {tx_hash.hex()}")
    print(f"  Explorer: https://polygonscan.com/tx/{tx_hash.hex()}")

    print(f"\nWaiting for confirmation...")
    tx_receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=120)

    if tx_receipt.status == 1:
        print(f"\nSUCCESS! USDC allowance has been set.")
        print(f"  Block: {tx_receipt.blockNumber}")
        print(f"  Gas Used: {tx_receipt.gasUsed}")
        print(f"\nYou can now place orders with the bot!")
    else:
        print(f"\nERROR: Transaction failed!")
        print(f"  Check transaction on Polygonscan: https://polygonscan.com/tx/{tx_hash.hex()}")
        exit(1)

except Exception as e:
    print(f"\nERROR sending transaction: {e}")
    exit(1)
