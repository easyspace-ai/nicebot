"""CTF Position Merging - Combine YES+NO tokens back to USDC."""
from web3 import Web3
from typing import Optional
from config import Config
from logger import logger


class CTFMerger:
    """Handles merging of complementary CTF positions."""

    def __init__(self):
        self.rpc_url = "https://polygon-rpc.com"
        self.w3 = Web3(Web3.HTTPProvider(self.rpc_url))

        if not self.w3.is_connected():
            raise Exception("Cannot connect to Polygon RPC")

        # Get wallet
        private_key = Config.PRIVATE_KEY
        self.account = self.w3.eth.account.from_key(private_key)
        self.wallet = self.account.address

        # Contract addresses
        self.CTF_ADDRESS = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"
        self.USDC_ADDRESS = "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"

        # ABI for mergePositions
        self.MERGE_ABI = [{
            "constant": False,
            "inputs": [
                {"name": "collateralToken", "type": "address"},
                {"name": "parentCollectionId", "type": "bytes32"},
                {"name": "conditionId", "type": "bytes32"},
                {"name": "partition", "type": "uint256[]"},
                {"name": "amount", "type": "uint256"}
            ],
            "name": "mergePositions",
            "outputs": [],
            "type": "function"
        }]

        self.ctf = self.w3.eth.contract(
            address=Web3.to_checksum_address(self.CTF_ADDRESS),
            abi=self.MERGE_ABI
        )

    def merge_positions(self, condition_id: str, amount: float) -> Optional[str]:
        """
        Merge complementary positions back to USDC.

        Args:
            condition_id: Market condition ID (0x...)
            amount: Number of complete sets to merge

        Returns:
            Transaction hash if successful, None otherwise
        """
        try:
            logger.info(f"Merging {amount} position sets for condition {condition_id[:16]}...")

            # Prepare parameters
            collateral_token = Web3.to_checksum_address(self.USDC_ADDRESS)
            parent_collection_id = b'\x00' * 32  # Null for Polymarket
            condition_id_bytes = bytes.fromhex(condition_id[2:])  # Remove '0x'
            partition = [1, 2]  # Binary market: YES and NO
            amount_wei = int(amount * 1e6)  # USDC has 6 decimals

            # Build transaction
            nonce = self.w3.eth.get_transaction_count(self.wallet)
            gas_price = self.w3.eth.gas_price

            merge_txn = self.ctf.functions.mergePositions(
                collateral_token,
                parent_collection_id,
                condition_id_bytes,
                partition,
                amount_wei
            ).build_transaction({
                'from': self.wallet,
                'nonce': nonce,
                'gas': 250000,  # Merge uses less gas than redeem
                'gasPrice': gas_price,
                'chainId': 137
            })

            # Estimate gas
            try:
                estimated_gas = self.w3.eth.estimate_gas(merge_txn)
                merge_txn['gas'] = int(estimated_gas * 1.2)
                logger.info(f"Estimated gas: {estimated_gas}")
            except Exception as e:
                logger.warning(f"Could not estimate gas: {e}, using default")

            # Sign and send
            signed_txn = self.w3.eth.account.sign_transaction(merge_txn, Config.PRIVATE_KEY)
            tx_hash = self.w3.eth.send_raw_transaction(signed_txn.raw_transaction)

            logger.info(f"Merge transaction sent: {tx_hash.hex()}")
            logger.info(f"PolygonScan: https://polygonscan.com/tx/{tx_hash.hex()}")

            # Wait for confirmation
            receipt = self.w3.eth.wait_for_transaction_receipt(tx_hash, timeout=120)

            if receipt.status == 1:
                logger.info(f"✓ Successfully merged {amount} position sets!")
                logger.info(f"Gas used: {receipt.gasUsed}")
                return tx_hash.hex()
            else:
                logger.error(f"✗ Merge transaction reverted")
                return None

        except Exception as e:
            logger.error(f"Error merging positions: {e}", exc_info=True)
            return None
