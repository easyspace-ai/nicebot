"""Automated redemption service for resolved markets."""
from web3 import Web3
from config import Config
from market_tracker import MarketTracker
from logger import logger
from typing import List

class AutoRedeemer:
    """Automatically redeem positions from resolved markets."""

    def __init__(self, private_key: str):
        self.private_key = private_key
        self.tracker = MarketTracker()

        # Initialize Web3
        self.w3 = Web3(Web3.HTTPProvider('https://polygon-rpc.com'))
        if not self.w3.is_connected():
            raise Exception("Cannot connect to Polygon RPC")

        # Get wallet address
        account = self.w3.eth.account.from_key(private_key)
        self.address = account.address

        # Contract addresses
        self.CTF_ADDRESS = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"
        self.USDC_ADDRESS = "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"

        # Contract ABI
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

    def redeem_market(self, market_slug: str, condition_id: str, auto_confirm: bool = False) -> bool:
        """Redeem positions for a specific market."""
        try:
            logger.info(f"Attempting to redeem market: {market_slug}")

            # Prepare redemption parameters
            collateral_token = Web3.to_checksum_address(self.USDC_ADDRESS)
            parent_collection_id = b'\x00' * 32  # Null for Polymarket
            condition_id_bytes = bytes.fromhex(condition_id[2:])  # Remove '0x'
            index_sets = [1, 2]  # Binary market: both outcomes

            logger.info(f"Condition ID: {condition_id}")
            logger.info(f"Index sets: {index_sets}")

            # Build transaction
            nonce = self.w3.eth.get_transaction_count(self.address)
            gas_price = self.w3.eth.gas_price

            redeem_txn = self.ctf.functions.redeemPositions(
                collateral_token,
                parent_collection_id,
                condition_id_bytes,
                index_sets
            ).build_transaction({
                'from': self.address,
                'nonce': nonce,
                'gas': 300000,
                'gasPrice': gas_price,
                'chainId': 137
            })

            # Estimate gas
            try:
                estimated_gas = self.w3.eth.estimate_gas(redeem_txn)
                redeem_txn['gas'] = int(estimated_gas * 1.2)
                logger.info(f"Estimated gas: {estimated_gas}")
            except Exception as e:
                logger.warning(f"Could not estimate gas: {e}")

            # Sign and send
            logger.info("Signing and sending redemption transaction...")
            signed_txn = self.w3.eth.account.sign_transaction(redeem_txn, self.private_key)
            tx_hash = self.w3.eth.send_raw_transaction(signed_txn.raw_transaction)

            logger.info(f"Transaction sent: {tx_hash.hex()}")
            logger.info(f"PolygonScan: https://polygonscan.com/tx/{tx_hash.hex()}")

            # Wait for confirmation
            receipt = self.w3.eth.wait_for_transaction_receipt(tx_hash, timeout=120)

            if receipt.status == 1:
                logger.info(f"✓ Successfully redeemed {market_slug}")
                logger.info(f"Gas used: {receipt.gasUsed}")

                # Remove from tracker
                self.tracker.remove_market(market_slug)
                return True
            else:
                logger.error(f"✗ Redemption failed for {market_slug}")
                return False

        except Exception as e:
            logger.error(f"Error redeeming {market_slug}: {e}", exc_info=True)
            return False

    def redeem_all_pending(self, auto_confirm: bool = True) -> int:
        """Redeem all pending positions."""
        unredeemed = self.tracker.get_unredeemed_markets()

        if not unredeemed:
            logger.info("No markets to redeem")
            return 0

        logger.info(f"Found {len(unredeemed)} market(s) to redeem")

        redeemed_count = 0
        for market in unredeemed:
            logger.info(f"\nRedeeming: {market.question}")
            success = self.redeem_market(market.market_slug, market.condition_id, auto_confirm)
            if success:
                redeemed_count += 1

        logger.info(f"\nRedeemed {redeemed_count}/{len(unredeemed)} markets")
        return redeemed_count

if __name__ == "__main__":
    print('='*60)
    print('Automated Redemption Service')
    print('='*60)

    redeemer = AutoRedeemer(Config.PRIVATE_KEY)
    print(f'Wallet: {redeemer.address}\n')

    # Check for pending redemptions
    count = redeemer.redeem_all_pending(auto_confirm=True)

    print('='*60)
    print(f'Redeemed {count} market(s)')
    print('='*60)
