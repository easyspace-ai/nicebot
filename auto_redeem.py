"""Automatic redemption of winning positions."""
from web3 import Web3
from config import Config
from logger import logger
import requests
from typing import Dict, List

class AutoRedeemer:
    """Automatically redeems winning positions."""

    def __init__(self):
        self.rpc_url = Config.RPC_URL
        self.w3 = Web3(Web3.HTTPProvider(self.rpc_url))

        if not self.w3.is_connected():
            raise Exception("Cannot connect to Polygon RPC")

        private_key = Config.PRIVATE_KEY
        self.account = self.w3.eth.account.from_key(private_key)
        self.wallet = self.account.address

        # Contract setup
        self.CTF_ADDRESS = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"
        self.USDC_ADDRESS = "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"

        self.REDEEM_ABI = [{
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

        self.ctf = self.w3.eth.contract(
            address=Web3.to_checksum_address(self.CTF_ADDRESS),
            abi=self.REDEEM_ABI
        )

    def check_and_redeem_all(self) -> int:
        """
        Check for redeemable positions and redeem them automatically.

        Returns:
            Number of positions successfully redeemed
        """
        try:
            # Fetch positions from Polymarket API
            api_url = f"https://data-api.polymarket.com/positions?user={self.wallet}"
            response = requests.get(api_url, timeout=10)

            if response.status_code != 200:
                logger.warning(f"Failed to fetch positions (status {response.status_code})")
                return 0

            positions = response.json()

            if not positions:
                return 0

            # Filter redeemable positions
            redeemable = [p for p in positions if p.get('redeemable', False)]

            if not redeemable:
                return 0

            # Group by condition ID
            by_condition: Dict[str, List] = {}
            for pos in redeemable:
                cid = pos['conditionId']
                if cid not in by_condition:
                    by_condition[cid] = []
                by_condition[cid].append(pos)

            total_value = sum(
                sum(p['currentValue'] for p in positions)
                for positions in by_condition.values()
            )

            logger.info(f"Found {len(redeemable)} redeemable positions worth ${total_value:.2f}")

            # Redeem each market
            success_count = 0

            for cid, positions in by_condition.items():
                market_title = positions[0]['title']
                market_value = sum(p['currentValue'] for p in positions)

                logger.info(f"Redeeming: {market_title} (${market_value:.2f})")

                if self._redeem_condition(cid):
                    success_count += 1

            if success_count > 0:
                logger.info(f"✓ Redeemed {success_count}/{len(by_condition)} markets")

            return success_count

        except Exception as e:
            logger.error(f"Error in check_and_redeem_all: {e}", exc_info=True)
            return 0

    def _redeem_condition(self, condition_id: str) -> bool:
        """Redeem a single condition."""
        try:
            collateral_token = Web3.to_checksum_address(self.USDC_ADDRESS)
            parent_collection_id = b'\x00' * 32
            condition_id_bytes = bytes.fromhex(condition_id[2:])
            index_sets = [1, 2]

            nonce = self.w3.eth.get_transaction_count(self.wallet)
            gas_price = self.w3.eth.gas_price

            redeem_txn = self.ctf.functions.redeemPositions(
                collateral_token,
                parent_collection_id,
                condition_id_bytes,
                index_sets
            ).build_transaction({
                'from': self.wallet,
                'nonce': nonce,
                'gas': 300000,
                'gasPrice': gas_price,
                'chainId': 137
            })

            # Estimate gas
            try:
                estimated_gas = self.w3.eth.estimate_gas(redeem_txn)
                redeem_txn['gas'] = int(estimated_gas * 1.2)
            except:
                pass

            signed_txn = self.w3.eth.account.sign_transaction(redeem_txn, Config.PRIVATE_KEY)
            tx_hash = self.w3.eth.send_raw_transaction(signed_txn.raw_transaction)

            receipt = self.w3.eth.wait_for_transaction_receipt(tx_hash, timeout=120)

            if receipt.status == 1:
                logger.info(f"  ✓ Redeemed! TX: {tx_hash.hex()}")
                return True
            else:
                logger.error(f"  ✗ Transaction reverted")
                return False

        except Exception as e:
            logger.error(f"  ✗ Error: {e}")
            return False
