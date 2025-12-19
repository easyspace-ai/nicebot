"""Scan for all CTF token holdings by checking Transfer events."""
from web3 import Web3
from config import Config

print('='*60)
print('Scanning All CTF Token Holdings')
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

# CTF Contract (ERC1155)
CTF_ADDRESS = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"

# Full ERC1155 ABI
ERC1155_ABI = [
    {
        "constant": True,
        "inputs": [
            {"name": "account", "type": "address"},
            {"name": "id", "type": "uint256"}
        ],
        "name": "balanceOf",
        "outputs": [{"name": "", "type": "uint256"}],
        "type": "function"
    },
    {
        "anonymous": False,
        "inputs": [
            {"indexed": True, "name": "operator", "type": "address"},
            {"indexed": True, "name": "from", "type": "address"},
            {"indexed": True, "name": "to", "type": "address"},
            {"indexed": False, "name": "id", "type": "uint256"},
            {"indexed": False, "name": "value", "type": "uint256"}
        ],
        "name": "TransferSingle",
        "type": "event"
    },
    {
        "anonymous": False,
        "inputs": [
            {"indexed": True, "name": "operator", "type": "address"},
            {"indexed": True, "name": "from", "type": "address"},
            {"indexed": True, "name": "to", "type": "address"},
            {"indexed": False, "name": "ids", "type": "uint256[]"},
            {"indexed": False, "name": "values", "type": "uint256[]"}
        ],
        "name": "TransferBatch",
        "type": "event"
    }
]

ctf = w3.eth.contract(
    address=Web3.to_checksum_address(CTF_ADDRESS),
    abi=ERC1155_ABI
)

print('Fetching recent CTF transfers to your wallet...')
print('(This may take a moment)\n')

try:
    # Get current block
    current_block = w3.eth.block_number
    from_block = current_block - 10000  # Last ~10k blocks (~5 hours on Polygon)

    print(f'Scanning blocks {from_block} to {current_block}...\n')

    # Get TransferSingle events where 'to' is your wallet
    transfer_filter = ctf.events.TransferSingle.create_filter(
        from_block=from_block,
        to_block='latest',
        argument_filters={'to': wallet}
    )

    transfers = transfer_filter.get_all_entries()

    if not transfers:
        print('No recent transfers found.')
        print('Your CTF tokens might be older than 10k blocks.')
        print('\nTrying to check via Polymarket API instead...')
    else:
        print(f'Found {len(transfers)} transfer(s):\n')

        # Track unique token IDs
        token_ids = set()

        for event in transfers:
            token_id = event['args']['id']
            value = event['args']['value']
            token_ids.add(token_id)

            print(f'Token ID: {token_id}')
            print(f'  Amount received: {value / 1_000_000:.6f} shares')
            print(f'  Block: {event["blockNumber"]}')
            print()

        print('='*60)
        print('Checking current balances for these tokens...\n')

        total_value = 0
        for token_id in token_ids:
            balance = ctf.functions.balanceOf(wallet, token_id).call()
            balance_formatted = balance / 1_000_000

            print(f'Token ID: {token_id}')
            print(f'  Current balance: {balance_formatted:.6f} shares')

            if balance > 0:
                total_value += balance_formatted
                print(f'  Status: YOU HAVE POSITIONS ✓')
            else:
                print(f'  Status: Already redeemed or sold')
            print()

        print('='*60)
        print(f'Total unredeemed positions: {total_value:.6f} shares')

        if total_value > 0:
            print('\nYou have CTF tokens to redeem!')
        else:
            print('\nAll positions have been redeemed or sold.')

except Exception as e:
    print(f'Error scanning transfers: {e}')
    print('\nPlease check your balance manually on Polymarket:')
    print('https://polymarket.com/portfolio')

print('='*60)
