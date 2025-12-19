"""Check all USDC and CTF allowances for Polymarket."""
from web3 import Web3
from config import Config

RPC_URL = Config.RPC_URL
USDC_ADDRESS = "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"
CTF_ADDRESS = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"

# All spenders
SPENDERS = [
    ("0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E", "CTF Exchange"),
    ("0xC5d563A36AE78145C45a50134d48A1215220f80a", "Neg Risk CTF Exchange"),
    ("0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296", "Neg Risk Adapter")
]

ERC20_ABI = [{
    "constant": True,
    "inputs": [
        {"name": "_owner", "type": "address"},
        {"name": "_spender", "type": "address"}
    ],
    "name": "allowance",
    "outputs": [{"name": "", "type": "uint256"}],
    "type": "function"
}]

ERC1155_ABI = [{
    "constant": True,
    "inputs": [
        {"name": "account", "type": "address"},
        {"name": "operator", "type": "address"}
    ],
    "name": "isApprovedForAll",
    "outputs": [{"name": "", "type": "bool"}],
    "type": "function"
}]

w3 = Web3(Web3.HTTPProvider(RPC_URL))
if not w3.is_connected():
    print("ERROR: Cannot connect to Polygon RPC")
    exit(1)

# Get wallet address
private_key = Config.PRIVATE_KEY
account = w3.eth.account.from_key(private_key)
wallet = account.address

print(f'Checking Allowances for Wallet: {wallet}\n')
print('='*70)

# Create contracts
usdc = w3.eth.contract(
    address=Web3.to_checksum_address(USDC_ADDRESS),
    abi=ERC20_ABI
)

ctf = w3.eth.contract(
    address=Web3.to_checksum_address(CTF_ADDRESS),
    abi=ERC1155_ABI
)

all_good = True

for spender_addr, spender_name in SPENDERS:
    print(f'\n{spender_name}:')
    print(f'  Address: {spender_addr}')

    spender_checksum = Web3.to_checksum_address(spender_addr)

    # Check USDC allowance
    usdc_allowance = usdc.functions.allowance(
        wallet,
        spender_checksum
    ).call()

    usdc_formatted = usdc_allowance / 1_000_000
    print(f'  USDC Allowance: ${usdc_formatted:,.2f}', end='')

    if usdc_allowance > 0:
        print(' [OK]')
    else:
        print(' [NOT SET]')
        all_good = False

    # Check CTF approval
    ctf_approved = ctf.functions.isApprovedForAll(
        wallet,
        spender_checksum
    ).call()

    print(f'  CTF Approved: {ctf_approved}', end='')

    if ctf_approved:
        print(' [OK]')
    else:
        print(' [NOT SET]')
        all_good = False

print('\n' + '='*70)
if all_good:
    print('[OK] All allowances are properly set!')
else:
    print('[ERROR] Some allowances are missing - run set_all_allowances.py')
