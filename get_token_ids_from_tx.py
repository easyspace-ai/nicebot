"""Get token IDs from transaction receipts."""
from web3 import Web3
from config import Config

print('='*60)
print('Extracting Token IDs from Transactions')
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

# Transaction hashes from screenshot (need full hashes)
# Based on visible parts: 0x01d760f7e2f... and 0x90a42ead6c...
# Let me try to get recent transactions to your wallet

print('Fetching recent transactions to your wallet...\n')

try:
    # Get latest block
    latest_block = w3.eth.block_number
    print(f'Latest block: {latest_block}')

    # Check last few blocks for transactions to your wallet
    print(f'Checking recent blocks for CTF transfers...\n')

    # CTF Contract
    CTF_ADDRESS = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"

    # TransferSingle event signature
    # keccak256("TransferSingle(address,address,address,uint256,uint256)")
    TRANSFER_SINGLE_TOPIC = "0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62"

    # Your wallet address as a topic (padded to 32 bytes)
    wallet_topic = '0x' + wallet[2:].lower().rjust(64, '0')

    # Look back 100 blocks (about 3-5 minutes on Polygon)
    from_block = latest_block - 100

    print(f'Scanning blocks {from_block} to {latest_block}...')

    # Get logs for TransferSingle events where 'to' is your wallet
    logs = w3.eth.get_logs({
        'fromBlock': from_block,
        'toBlock': 'latest',
        'address': CTF_ADDRESS,
        'topics': [
            TRANSFER_SINGLE_TOPIC,  # TransferSingle event
            None,  # operator (any)
            None,  # from (any)
            wallet_topic  # to (your wallet)
        ]
    })

    if not logs:
        print('No recent TransferSingle events found.')
        print('The tokens might be from older transactions.')
    else:
        print(f'\nFound {len(logs)} transfer(s):\n')

        token_ids = []

        for log in logs:
            tx_hash = log['transactionHash'].hex()
            block = log['blockNumber']

            # Decode the data field (contains id and value)
            # Data layout: [id (32 bytes)][value (32 bytes)]
            data = log['data']

            # Parse token ID (first 32 bytes after 0x)
            token_id = int(data[2:66], 16)
            # Parse value (next 32 bytes)
            value = int(data[66:130], 16)

            token_ids.append(token_id)

            print(f'Transaction: {tx_hash}')
            print(f'  Block: {block}')
            print(f'  Token ID: {token_id}')
            print(f'  Amount: {value / 1_000_000:.6f} shares')
            print()

        # Now check balances for these tokens
        print('='*60)
        print('Checking current balances...\n')

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

        total_balance = 0
        tokens_to_redeem = []

        for token_id in set(token_ids):
            balance = ctf.functions.balanceOf(wallet, token_id).call()
            balance_formatted = balance / 1_000_000

            print(f'Token ID: {token_id}')
            print(f'  Balance: {balance_formatted:.6f} shares')

            if balance > 0:
                total_balance += balance_formatted
                tokens_to_redeem.append((token_id, balance))
                print(f'  Status: YOU HAVE POSITIONS ✓')
            else:
                print(f'  Status: Already redeemed')
            print()

        print('='*60)
        print(f'Total balance: {total_balance:.6f} shares')

        if tokens_to_redeem:
            print(f'\nYou have {len(tokens_to_redeem)} token(s) to redeem!')
            print('\nSave these token IDs - we\'ll use them to redeem:')
            for token_id, balance in tokens_to_redeem:
                print(f'  - {token_id} ({balance / 1_000_000:.6f} shares)')

except Exception as e:
    print(f'Error: {e}')
    import traceback
    traceback.print_exc()

print('\n' + '='*60)
