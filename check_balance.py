"""Check actual USDC balance on Polygon."""
from web3 import Web3

w3 = Web3(Web3.HTTPProvider('https://polygon-rpc.com'))
usdc_address = '0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174'
wallet = '0x6eA221C3a41c76E90D1cdAA01BCF6922171Eb46E'

usdc = w3.eth.contract(
    address=usdc_address,
    abi=[{
        'constant': True,
        'inputs': [{'name': '_owner', 'type': 'address'}],
        'name': 'balanceOf',
        'outputs': [{'name': 'balance', 'type': 'uint256'}],
        'type': 'function'
    }]
)

balance_raw = usdc.functions.balanceOf(wallet).call()
balance_usdc = balance_raw / 1_000_000

print(f'Wallet: {wallet}')
print(f'USDC Balance (on-chain): ${balance_usdc:.2f}')

if balance_usdc == 0:
    print('\nERROR: No USDC in wallet!')
    print('Your wallet does not actually have any USDC on Polygon.')
    print('\nTo fix:')
    print('1. Get USDC on Polygon network')
    print('2. Send at least $10 USDC to this address')
    print('   Use a bridge or send from another Polygon wallet')
elif balance_usdc < 10:
    print(f'\nWARNING: Balance too low (${balance_usdc:.2f} < $10.00)')
else:
    print('\nBalance is sufficient for trading!')
