"""Check conditional token positions."""
from web3 import Web3
from config import Config

print('='*60)
print('Checking Conditional Token Positions')
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

# CTF Contract (ERC1155)
CTF_ADDRESS = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"

# The two token IDs from your recent orders
TOKEN_IDS = [
    "6899861987000015801984996681312331819779392056725229078736800777968742483463",  # Up
    "67115365398658057765454959718117326110485817467272760228424903817929064999811"   # Down
]

# ERC1155 balanceOf ABI
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

print('Checking token balances...\n')

total_positions = 0

for i, token_id in enumerate(TOKEN_IDS):
    outcome = "Up" if i == 0 else "Down"

    try:
        balance = ctf.functions.balanceOf(wallet, int(token_id)).call()
        balance_formatted = balance / 1_000_000  # 6 decimals

        print(f'{outcome} tokens (ID: ...{token_id[-8:]}):')
        print(f'  Balance: {balance_formatted:.6f} shares')

        if balance > 0:
            total_positions += balance_formatted
            print(f'  Status: YOU HAVE POSITIONS')
        else:
            print(f'  Status: No positions')
        print()

    except Exception as e:
        print(f'ERROR checking {outcome}: {e}\n')

print('='*60)
print(f'Total positions: {total_positions:.6f} shares')

if total_positions > 0:
    print('\nYou have conditional token positions!')
    print('If the market resolved in your favor, you can redeem them.')
else:
    print('\nNo conditional token positions found.')
    print('Your orders may still be open or were cancelled.')

print('='*60)
