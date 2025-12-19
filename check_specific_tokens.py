"""Check specific CTF token balances from PolygonScan."""
from web3 import Web3
from config import Config

print('='*60)
print('Checking Specific CTF Token Balances')
print('='*60)

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

# CTF Contract
CTF_ADDRESS = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"

# Based on the screenshots, these are truncated - we need full token IDs
# Let me fetch the actual transaction to get the full token IDs

# Transaction hashes from the screenshot
TX_HASHES = [
    "0x01d760f7e2f...",  # First one (25 mins ago)
    "0x90a42ead6c..."   # Second one (27 mins ago)
]

print('We need to get the full token IDs from the transactions.')
print('Please provide the complete transaction hashes from PolygonScan.\n')

print('Alternative: Check the transaction details and look for:')
print('- In the "Logs" section')
print('- Find "TransferSingle" events')
print('- Look for the "id" field (this is the token ID)')
print()

# If you can provide the full token IDs, we can check them like this:
"""
# Example token IDs (replace with actual values)
TOKEN_IDS = [
    31422594496434...,  # Full number needed
    112414748075843...  # Full number needed
]

ERC1155_ABI = [{
    "constant": True,
    "inputs": [
        {"name": "account", "type": "address"},
        {"name": "id", "type": "uint256"}
    ],
    "name": "balanceOf",
    "outputs": [{"name": "", "type": "uint256"}],
    "type": "function"
}]

ctf = w3.eth.contract(
    address=Web3.to_checksum_address(CTF_ADDRESS),
    abi=ERC1155_ABI
)

for token_id in TOKEN_IDS:
    balance = ctf.functions.balanceOf(wallet, token_id).call()
    print(f'Token ID: {token_id}')
    print(f'  Balance: {balance / 1_000_000:.6f} shares')
    print()
"""

print('='*60)
