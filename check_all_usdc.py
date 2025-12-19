"""Check all USDC token balances."""
from web3 import Web3

w3 = Web3(Web3.HTTPProvider('https://polygon-rpc.com'))
wallet = '0x6eA221C3a41c76E90D1cdAA01BCF6922171Eb46E'

# USDC.e (bridged - what Polymarket uses)
usdc_e = w3.eth.contract(
    address='0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174',
    abi=[{'constant': True, 'inputs': [{'name': '_owner', 'type': 'address'}], 'name': 'balanceOf', 'outputs': [{'name': 'balance', 'type': 'uint256'}], 'type': 'function'}]
)

# USDC (native)
usdc_native = w3.eth.contract(
    address='0x3c499c542cEF5E3811e1192ce70d8cC03d5c3359',
    abi=[{'constant': True, 'inputs': [{'name': '_owner', 'type': 'address'}], 'name': 'balanceOf', 'outputs': [{'name': 'balance', 'type': 'uint256'}], 'type': 'function'}]
)

usdc_e_bal = usdc_e.functions.balanceOf(wallet).call()
usdc_native_bal = usdc_native.functions.balanceOf(wallet).call()

print(f'Wallet: {wallet}\n')
print(f'USDC.e (bridged) - Polymarket uses this: ${usdc_e_bal / 1_000_000:.6f}')
print(f'USDC (native) - You have this:           ${usdc_native_bal / 1_000_000:.6f}')
print()

if usdc_native_bal > 0 and usdc_e_bal == 0:
    print('PROBLEM: You have native USDC but Polymarket needs USDC.e')
    print()
    print('Solution: Swap USDC to USDC.e using QuickSwap or Uniswap')
    print('  1. Go to https://quickswap.exchange/')
    print('  2. Connect wallet')
    print(f'  3. Swap USDC ({usdc_native_bal / 1_000_000:.2f}) to USDC.e')
    print('  4. Or I can create a swap transaction for you')
elif usdc_e_bal > 0:
    print(f'SUCCESS: You have ${usdc_e_bal / 1_000_000:.2f} USDC.e')
    print('This is the correct token for Polymarket!')
else:
    print('No USDC found in wallet')
