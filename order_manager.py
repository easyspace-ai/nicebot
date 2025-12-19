"""Order management for Polymarket limit orders."""

import time
from datetime import datetime
from typing import List, Optional, Dict, Tuple
from decimal import Decimal
from py_clob_client.client import ClobClient
from py_clob_client.clob_types import OrderArgs, OrderType, ApiCreds
from py_clob_client.constants import POLYGON
from models import Market, OrderRecord, OrderSide, OrderStatus, Outcome
from logger import logger
from config import Config


class OrderManager:
    """Manages order placement, tracking, and cancellation."""

    def __init__(self, private_key: str):
        """
        Initialize order manager with Polymarket CLOB client.

        Args:
            private_key: Private key for wallet authentication
        """
        self.private_key = private_key
        self.client = None
        self.address = None
        self._initialize_client()

    def _initialize_client(self):
        """Initialize the CLOB client and set allowances."""
        try:
            # Initialize client based on signature type
            host = Config.CLOB_API_URL
            chain_id = Config.CHAIN_ID

            if Config.SIGNATURE_TYPE == "EOA":
                # Initialize client with private key
                self.client = ClobClient(
                    host=host,
                    key=self.private_key,
                    chain_id=chain_id
                )

                # Derive API credentials from private key
                # This generates credentials that match the wallet address
                try:
                    logger.info("Deriving API credentials from private key...")
                    creds = self.client.create_or_derive_api_creds()
                    self.client.set_api_creds(creds)
                    logger.info("API credentials derived and set successfully")
                except Exception as e:
                    logger.warning(f"Could not derive API credentials: {e}")
                    logger.warning("Will continue without L2 authentication (read-only mode)")
            elif Config.SIGNATURE_TYPE in ["POLY_PROXY", "POLY_GNOSIS_SAFE"]:
                if not Config.FUNDER_ADDRESS:
                    raise ValueError("FUNDER_ADDRESS required for proxy wallets")

                self.client = ClobClient(
                    host=host,
                    key=self.private_key,
                    chain_id=chain_id,
                    signature_type=1 if Config.SIGNATURE_TYPE == "POLY_PROXY" else 2,
                    funder=Config.FUNDER_ADDRESS
                )
            else:
                raise ValueError(f"Invalid SIGNATURE_TYPE: {Config.SIGNATURE_TYPE}")

            # Derive address
            self.address = self.client.get_address()
            logger.info(f"Initialized CLOB client for address: {self.address}")

            # Set allowances for USDC and CTF exchange
            self._set_allowances()

        except Exception as e:
            logger.error(f"Failed to initialize CLOB client: {e}", exc_info=True)
            raise

    def _set_allowances(self):
        """Set token allowances for trading."""
        try:
            logger.info("Setting token allowances...")

            # Update balance allowance for the exchange (new API)
            try:
                result = self.client.update_balance_allowance()
                logger.info("Balance allowance updated successfully")
            except Exception as e:
                logger.warning(f"Could not update allowances: {e}")
                logger.info("Allowances may already be set or need manual setup")

        except Exception as e:
            logger.error(f"Error setting allowances: {e}")

    def get_usdc_balance(self) -> float:
        """Get USDC balance directly from wallet on Polygon blockchain."""
        try:
            from web3 import Web3

            # Connect to Polygon RPC
            w3 = Web3(Web3.HTTPProvider(Config.RPC_URL))
            if not w3.is_connected():
                logger.error("Cannot connect to Polygon RPC")
                return 0.0

            # USDC contract on Polygon
            usdc_address = "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"
            balance_abi = [{
                "constant": True,
                "inputs": [{"name": "_owner", "type": "address"}],
                "name": "balanceOf",
                "outputs": [{"name": "balance", "type": "uint256"}],
                "type": "function"
            }]

            usdc_contract = w3.eth.contract(
                address=Web3.to_checksum_address(usdc_address),
                abi=balance_abi
            )

            # Query balance
            balance_wei = usdc_contract.functions.balanceOf(self.address).call()
            usdc_balance = balance_wei / 1e6  # USDC has 6 decimals

            logger.debug(f"USDC balance from wallet: ${usdc_balance:.2f}")
            return usdc_balance
        except Exception as e:
            logger.error(f"Error getting USDC balance from wallet: {e}", exc_info=True)
            return 0.0

    def update_market_prices(self, market: Market) -> Market:
        """
        Update market with current orderbook prices.

        Args:
            market: Market to update

        Returns:
            Updated market with current prices
        """
        try:
            for outcome in market.outcomes:
                if not outcome.token_id:
                    continue

                # Get orderbook
                try:
                    book = self.client.get_order_book(outcome.token_id)

                    # Extract best bid and ask from OrderBookSummary object
                    if hasattr(book, 'bids') and book.bids:
                        outcome.best_bid = float(book.bids[0].price) if hasattr(book.bids[0], 'price') else 0
                    if hasattr(book, 'asks') and book.asks:
                        outcome.best_ask = float(book.asks[0].price) if hasattr(book.asks[0], 'price') else 0

                    # Set mid price
                    if outcome.best_bid and outcome.best_ask:
                        outcome.price = (outcome.best_bid + outcome.best_ask) / 2

                    logger.debug(
                        f"{outcome.outcome}: bid={outcome.best_bid}, "
                        f"ask={outcome.best_ask}"
                    )

                except Exception as e:
                    logger.warning(f"Could not get orderbook for {outcome.token_id}: {e}")

            return market

        except Exception as e:
            logger.error(f"Error updating market prices: {e}")
            return market

    def calculate_order_size(self, price: float, usd_amount: float) -> float:
        """
        Calculate order size in outcome shares.

        Args:
            price: Order price
            usd_amount: USD amount to trade

        Returns:
            Number of shares
        """
        if price <= 0:
            return 0.0
        shares = usd_amount / price
        # Round to 2 decimal places for shares
        return round(shares, 2)

    def place_simple_test_orders(
        self,
        market: Market,
        price: float = 0.49,
        size: float = 10.0
    ) -> List[OrderRecord]:
        """
        Place 2 simple test orders: 1 on Yes, 1 on No at fixed price/size.

        Args:
            market: Market to place orders on
            price: Fixed price for both orders (default: 0.49)
            size: Fixed size in shares (default: 10.0)

        Returns:
            List of order records
        """
        placed_orders = []

        try:
            # Check balance first (skip if API call fails)
            balance = self.get_usdc_balance()
            required_balance = price * size * 2  # 2 orders at fixed price/size

            if balance > 0 and balance < required_balance:
                logger.error(
                    f"Insufficient balance: ${balance:.2f} < ${required_balance:.2f}"
                )
                return placed_orders
            elif balance == 0:
                logger.warning("Could not check balance, proceeding with order placement anyway")

            # Update market prices (for logging purposes)
            market = self.update_market_prices(market)

            # Find Yes/No or Up/Down outcomes
            yes_outcome = None
            no_outcome = None

            logger.info(f"Market has {len(market.outcomes)} outcomes")
            for outcome in market.outcomes:
                logger.info(f"  Outcome: '{outcome.outcome}' (token_id: {outcome.token_id})")
                outcome_upper = outcome.outcome.upper()
                if outcome_upper in ["YES", "UP"]:
                    yes_outcome = outcome
                elif outcome_upper in ["NO", "DOWN"]:
                    no_outcome = outcome

            if not yes_outcome or not no_outcome:
                logger.error("Could not find both outcomes (Yes/No or Up/Down)")
                return placed_orders

            if not yes_outcome.token_id or not no_outcome.token_id:
                logger.error("Missing token IDs for outcomes")
                return placed_orders

            # Place order on Yes
            yes_order = self._place_single_order_fixed(
                market=market,
                outcome=yes_outcome,
                side=OrderSide.BUY,
                price=price,
                size=size
            )
            if yes_order:
                placed_orders.append(yes_order)

            # Small delay between orders
            time.sleep(0.5)

            # Place order on No
            no_order = self._place_single_order_fixed(
                market=market,
                outcome=no_outcome,
                side=OrderSide.BUY,
                price=price,
                size=size
            )
            if no_order:
                placed_orders.append(no_order)

            logger.info(
                f"Placed {len(placed_orders)} test orders for market {market.market_slug} "
                f"at ${price:.2f} with {size:.2f} shares each"
            )

        except Exception as e:
            logger.error(f"Error placing test orders: {e}", exc_info=True)

        return placed_orders

    def place_liquidity_orders(self, market: Market) -> List[OrderRecord]:
        """
        Place two-sided liquidity orders for a market.

        Places buy and sell orders on both Yes and No outcomes.

        Args:
            market: Market to place orders on

        Returns:
            List of order records
        """
        placed_orders = []

        try:
            # Check balance first
            balance = self.get_usdc_balance()
            # Only need USDC for BUY orders (2 outcomes × 1 BUY side each)
            # SELL limit orders would require tokens we don't have yet
            required_balance = Config.ORDER_SIZE_USD * 2

            if balance < required_balance:
                logger.error(
                    f"Insufficient balance: ${balance:.2f} < ${required_balance:.2f}"
                )
                return placed_orders

            # Update market prices
            market = self.update_market_prices(market)

            # Place orders for each outcome
            for outcome in market.outcomes:
                if not outcome.token_id or not outcome.best_bid or not outcome.best_ask:
                    logger.warning(f"Missing data for {outcome.outcome}, skipping")
                    continue

                # Calculate order prices
                buy_price = self._adjust_price(
                    outcome.best_bid - Config.SPREAD_OFFSET,
                    is_buy=True
                )
                sell_price = self._adjust_price(
                    outcome.best_ask + Config.SPREAD_OFFSET,
                    is_buy=False
                )

                # Place buy order
                buy_order = self._place_single_order(
                    market=market,
                    outcome=outcome,
                    side=OrderSide.BUY,
                    price=buy_price
                )
                if buy_order:
                    placed_orders.append(buy_order)

                # Small delay between orders
                time.sleep(0.5)

                # Place sell order
                sell_order = self._place_single_order(
                    market=market,
                    outcome=outcome,
                    side=OrderSide.SELL,
                    price=sell_price
                )
                if sell_order:
                    placed_orders.append(sell_order)

                time.sleep(0.5)

            logger.info(
                f"Placed {len(placed_orders)} orders for market {market.market_slug}"
            )

            # VERIFY orders are actually in the orderbook
            if placed_orders:
                verified_orders = self.verify_orders_in_orderbook(
                    market.market_slug,
                    market.condition_id,
                    placed_orders
                )

                # Log final status
                success_count = sum(1 for o in verified_orders if o.status == OrderStatus.PLACED)
                failed_count = sum(1 for o in verified_orders if o.status == OrderStatus.FAILED)

                logger.info(f"Order verification complete: {success_count} placed, {failed_count} failed")
                for order in verified_orders:
                    status_symbol = "✓" if order.status == OrderStatus.PLACED else "✗"
                    logger.info(
                        f"  {status_symbol} {order.side.value} {order.outcome} @ ${order.price:.2f} "
                        f"x {order.size} shares - {order.status.value}"
                    )

                placed_orders = verified_orders

        except Exception as e:
            logger.error(f"Error placing liquidity orders: {e}", exc_info=True)

        return placed_orders

    def _adjust_price(self, price: float, is_buy: bool) -> float:
        """
        Adjust price to valid range and tick size.

        Args:
            price: Raw price
            is_buy: Whether this is a buy order

        Returns:
            Adjusted price
        """
        # Clamp to valid range [0.01, 0.99]
        price = max(0.01, min(0.99, price))

        # Round to nearest 0.01 (tick size)
        price = round(price, 2)

        return price

    def _place_single_order_fixed(
        self,
        market: Market,
        outcome: Outcome,
        side: OrderSide,
        price: float,
        size: float
    ) -> Optional[OrderRecord]:
        """Place a single limit order with fixed price and size."""
        try:
            if size <= 0:
                logger.error(f"Invalid order size: {size}")
                return None

            size_usd = price * size

            logger.info(
                f"Placing {side.value} order: {outcome.outcome} @ ${price:.2f} "
                f"for {size:.2f} shares (${size_usd:.2f})"
            )

            # Create order arguments (OrderArgs doesn't accept order_type, GTC is default)
            order_args = OrderArgs(
                token_id=outcome.token_id,
                price=price,
                size=size,
                side=side.value
            )

            # Create and sign order
            signed_order = self.client.create_order(order_args)

            # Post order to Polymarket orderbook
            post_response = self.client.post_order(signed_order)

            # Extract order ID from post response
            order_id = ""
            if isinstance(post_response, dict):
                order_id = post_response.get("orderID", "")
            elif hasattr(post_response, 'orderID'):
                order_id = post_response.orderID
            elif hasattr(signed_order, 'order') and hasattr(signed_order.order, 'salt'):
                # Fallback: use salt from signed order
                order_id = str(signed_order.order.salt)

            if not order_id:
                logger.error(f"No order ID in post response: {post_response}")
                return OrderRecord(
                    order_id="FAILED",
                    market_slug=market.market_slug,
                    condition_id=market.condition_id,
                    token_id=outcome.token_id,
                    outcome=outcome.outcome,
                    side=side,
                    price=price,
                    size=size,
                    size_usd=size_usd,
                    status=OrderStatus.FAILED,
                    error_message=f"No order ID in post response"
                )

            logger.info(f"Order posted successfully to orderbook: {order_id}")

            return OrderRecord(
                order_id=order_id,
                market_slug=market.market_slug,
                condition_id=market.condition_id,
                token_id=outcome.token_id,
                outcome=outcome.outcome,
                side=side,
                price=price,
                size=size,
                size_usd=size_usd,
                status=OrderStatus.PLACED
            )

        except Exception as e:
            logger.error(f"Error placing order: {e}", exc_info=True)

            # Check if this is a balance/allowance error
            error_str = str(e).lower()
            is_balance_error = 'balance' in error_str or 'allowance' in error_str

            if is_balance_error:
                logger.error("❌ INSUFFICIENT BALANCE OR ALLOWANCE - Cannot place orders!")

            # Check if order was actually signed (may still be in orderbook despite API error)
            order_id = None
            if 'signed_order' in locals() and hasattr(signed_order, 'order'):
                if hasattr(signed_order.order, 'salt'):
                    order_id = str(signed_order.order.salt)
                    logger.warning(f"API error but order was signed - may still be in orderbook: {order_id}")

                    # Return as potentially placed (verification will check orderbook)
                    return OrderRecord(
                        order_id=order_id,
                        market_slug=market.market_slug,
                        condition_id=market.condition_id,
                        token_id=outcome.token_id,
                        outcome=outcome.outcome,
                        side=side,
                        price=price,
                        size=size,
                        size_usd=price * size,
                        status=OrderStatus.PLACED,  # Will be verified by verify_orders_in_orderbook()
                        error_message=f"API error (will verify): {e}"
                    )

            # If we couldn't get signed order, truly failed
            return OrderRecord(
                order_id="FAILED",
                market_slug=market.market_slug,
                condition_id=market.condition_id,
                token_id=outcome.token_id,
                outcome=outcome.outcome,
                side=side,
                price=price,
                size=0,
                size_usd=price * size,
                status=OrderStatus.FAILED,
                error_message=str(e)
            )

    def _place_single_order(
        self,
        market: Market,
        outcome: Outcome,
        side: OrderSide,
        price: float
    ) -> Optional[OrderRecord]:
        """Place a single limit order."""
        try:
            # Calculate size
            size = self.calculate_order_size(price, Config.ORDER_SIZE_USD)

            if size <= 0:
                logger.error(f"Invalid order size: {size}")
                return None

            logger.info(
                f"Placing {side.value} order: {outcome.outcome} @ ${price:.2f} "
                f"for {size:.2f} shares (${Config.ORDER_SIZE_USD})"
            )

            # Create order arguments (OrderArgs doesn't accept order_type, GTC is default)
            order_args = OrderArgs(
                token_id=outcome.token_id,
                price=price,
                size=size,
                side=side.value
            )

            # Create and sign order
            signed_order = self.client.create_order(order_args)

            # Post order to Polymarket orderbook
            post_response = self.client.post_order(signed_order)

            # Extract order ID from post response
            order_id = ""
            if isinstance(post_response, dict):
                order_id = post_response.get("orderID", "")
            elif hasattr(post_response, 'orderID'):
                order_id = post_response.orderID
            elif hasattr(signed_order, 'order') and hasattr(signed_order.order, 'salt'):
                # Fallback: use salt from signed order
                order_id = str(signed_order.order.salt)

            if not order_id:
                logger.error(f"No order ID in post response: {post_response}")
                return OrderRecord(
                    order_id="FAILED",
                    market_slug=market.market_slug,
                    condition_id=market.condition_id,
                    token_id=outcome.token_id,
                    outcome=outcome.outcome,
                    side=side,
                    price=price,
                    size=size,
                    size_usd=Config.ORDER_SIZE_USD,
                    status=OrderStatus.FAILED,
                    error_message=f"No order ID in post response"
                )

            logger.info(f"Order posted successfully to orderbook: {order_id}")

            return OrderRecord(
                order_id=order_id,
                market_slug=market.market_slug,
                condition_id=market.condition_id,
                token_id=outcome.token_id,
                outcome=outcome.outcome,
                side=side,
                price=price,
                size=size,
                size_usd=Config.ORDER_SIZE_USD,
                status=OrderStatus.PLACED
            )

        except Exception as e:
            logger.error(f"Error placing order: {e}", exc_info=True)
            return OrderRecord(
                order_id="FAILED",
                market_slug=market.market_slug,
                condition_id=market.condition_id,
                token_id=outcome.token_id,
                outcome=outcome.outcome,
                side=side,
                price=price,
                size=0,
                size_usd=Config.ORDER_SIZE_USD,
                status=OrderStatus.FAILED,
                error_message=str(e)
            )

    def check_order_status(self, order: OrderRecord) -> OrderRecord:
        """
        Check and update order status.

        Args:
            order: Order to check

        Returns:
            Updated order record
        """
        try:
            # Get order details
            order_details = self.client.get_order(order.order_id)

            if not order_details:
                logger.warning(f"Could not get details for order {order.order_id}")
                return order

            # Check status
            status = order_details.get("status", "").upper()
            size_matched = float(order_details.get("size_matched", 0))
            original_size = float(order_details.get("original_size", order.size))

            if status == "MATCHED" or size_matched >= original_size:
                order.status = OrderStatus.FILLED
                if not order.filled_at:
                    order.filled_at = datetime.now()
                logger.info(f"Order {order.order_id} filled completely")

            elif size_matched > 0:
                order.status = OrderStatus.PARTIALLY_FILLED
                logger.info(
                    f"Order {order.order_id} partially filled: "
                    f"{size_matched}/{original_size}"
                )

            elif status == "CANCELLED":
                order.status = OrderStatus.CANCELLED
                logger.info(f"Order {order.order_id} cancelled")

        except Exception as e:
            logger.error(f"Error checking order status for {order.order_id}: {e}")

        return order

    def verify_orders_in_orderbook(self, market_slug: str, condition_id: str, placed_orders: List[OrderRecord]) -> List[OrderRecord]:
        """
        Verify which orders actually made it to the orderbook.

        Args:
            market_slug: Market identifier
            condition_id: Market condition ID
            placed_orders: List of OrderRecords from order placement attempt

        Returns:
            Updated list of OrderRecords with corrected statuses
        """
        try:
            logger.info(f"Verifying orders for {market_slug} in orderbook...")

            # Get all active orders for this market from the orderbook
            from py_clob_client.client import OpenOrderParams
            active_orders = self.client.get_orders(OpenOrderParams(market=condition_id))

            # Create a set of active order IDs for quick lookup
            active_order_ids = {order.get('id') for order in active_orders if order.get('id')}

            logger.info(f"Found {len(active_orders)} active orders in orderbook for market")

            # Update order statuses based on what's actually in orderbook
            verified_orders = []
            for order in placed_orders:
                if order.order_id in active_order_ids:
                    # Order is confirmed in orderbook
                    order.status = OrderStatus.PLACED
                    order.error_message = None
                    logger.info(f"✓ Verified order {order.order_id[:16]}... in orderbook")
                else:
                    # Order NOT in orderbook - mark as failed
                    order.status = OrderStatus.FAILED
                    order.size = 0
                    order.size_usd = 0
                    if not order.error_message:
                        order.error_message = "Order not found in orderbook after placement"
                    logger.warning(f"✗ Order {order.order_id[:16]}... NOT in orderbook - marking as FAILED")

                verified_orders.append(order)

            return verified_orders

        except Exception as e:
            logger.error(f"Error verifying orders in orderbook: {e}", exc_info=True)
            # On error, return orders as-is (don't change their status)
            return placed_orders

    def cancel_order(self, order_id: str) -> bool:
        """
        Cancel an order.

        Args:
            order_id: Order ID to cancel

        Returns:
            True if successful
        """
        try:
            logger.info(f"Cancelling order {order_id}")
            response = self.client.cancel_order(order_id)
            logger.info(f"Order cancelled: {order_id}")
            return True
        except Exception as e:
            logger.error(f"Error cancelling order {order_id}: {e}")
            return False

    def cancel_orders(self, orders: List[OrderRecord]) -> int:
        """
        Cancel multiple orders.

        Args:
            orders: List of orders to cancel

        Returns:
            Number of successfully cancelled orders
        """
        cancelled_count = 0
        for order in orders:
            if order.status in [OrderStatus.PLACED, OrderStatus.PARTIALLY_FILLED]:
                if self.cancel_order(order.order_id):
                    cancelled_count += 1
                    order.status = OrderStatus.CANCELLED
                time.sleep(0.3)  # Rate limiting

        return cancelled_count

    def get_positions(self, token_ids: List[str]) -> Dict[str, float]:
        """
        Get current positions for token IDs.

        Args:
            token_ids: List of token IDs to check

        Returns:
            Dict mapping token_id to position size
        """
        positions = {}
        try:
            # Get all positions from CLOB
            # Note: This may need to query the contract directly or use API
            # For now, we'll return empty - this needs the actual CLOB API method
            logger.warning("Position checking not fully implemented yet")
            return positions
        except Exception as e:
            logger.error(f"Error getting positions: {e}")
            return positions

    def sell_position_market(
        self,
        market: Market,
        outcome: Outcome,
        size: float
    ) -> Optional[OrderRecord]:
        """
        Sell a position at market price (current best bid).

        Args:
            market: Market to sell in
            outcome: Outcome to sell
            size: Size to sell in shares

        Returns:
            Order record if successful
        """
        try:
            if size <= 0:
                logger.warning(f"Cannot sell size {size}")
                return None

            # Get current best bid (we'll sell at this price to get filled immediately)
            market = self.update_market_prices(market)

            # Find the outcome
            target_outcome = None
            for o in market.outcomes:
                if o.token_id == outcome.token_id:
                    target_outcome = o
                    break

            if not target_outcome or not target_outcome.best_bid:
                logger.error(f"No best bid available for {outcome.outcome}")
                return None

            # Sell at best bid to ensure quick fill
            sell_price = self._adjust_price(target_outcome.best_bid, is_buy=False)

            logger.info(
                f"Selling position: {outcome.outcome} @ ${sell_price:.2f} "
                f"for {size:.2f} shares (market order)"
            )

            # Create sell order (OrderArgs doesn't accept order_type, GTC is default)
            order_args = OrderArgs(
                token_id=outcome.token_id,
                price=sell_price,
                size=size,
                side=OrderSide.SELL.value
            )

            # Create and sign order
            signed_order = self.client.create_order(order_args)

            # Post order to Polymarket orderbook
            post_response = self.client.post_order(signed_order)

            # Extract order ID from post response
            order_id = ""
            if isinstance(post_response, dict):
                order_id = post_response.get("orderID", "")
            elif hasattr(post_response, 'orderID'):
                order_id = post_response.orderID
            elif hasattr(signed_order, 'order') and hasattr(signed_order.order, 'salt'):
                # Fallback: use salt from signed order
                order_id = str(signed_order.order.salt)

            if not order_id:
                logger.error(f"No order ID in post response: {post_response}")
                return None

            logger.info(f"Market sell order posted to orderbook: {order_id}")

            return OrderRecord(
                order_id=order_id,
                market_slug=market.market_slug,
                condition_id=market.condition_id,
                token_id=outcome.token_id,
                outcome=outcome.outcome,
                side=OrderSide.SELL,
                price=sell_price,
                size=size,
                size_usd=sell_price * size,
                status=OrderStatus.PLACED
            )

        except Exception as e:
            logger.error(f"Error selling position: {e}", exc_info=True)
            return None

    def merge_positions_if_possible(
        self,
        market: Market,
        orders: List[OrderRecord],
        already_merged_amount: float = 0.0
    ) -> float:
        """
        Merge complementary positions (YES+NO) back to USDC if bot holds both.

        Args:
            market: Market information
            orders: List of orders for this market
            already_merged_amount: Amount already merged for this market

        Returns:
            Amount merged (0 if none)
        """
        try:
            from ctf_merge import CTFMerger
            from typing import Dict

            # Check filled amounts for each outcome
            filled_by_outcome: Dict[str, float] = {"YES": 0.0, "NO": 0.0}

            for order in orders:
                if order.status in [OrderStatus.FILLED, OrderStatus.PARTIALLY_FILLED]:
                    # Get actual filled size from API
                    try:
                        order_details = self.client.get_order(order.order_id)
                        size_matched = float(order_details.get("size_matched", 0))

                        if size_matched > 0:
                            outcome_upper = order.outcome.strip().upper()
                            if outcome_upper in ["YES", "UP"]:
                                filled_by_outcome["YES"] += size_matched
                            elif outcome_upper in ["NO", "DOWN"]:
                                filled_by_outcome["NO"] += size_matched
                    except Exception as e:
                        logger.warning(f"Could not get filled size for order {order.order_id}: {e}")

            # Check if we have both YES and NO positions
            yes_amount = filled_by_outcome.get("YES", 0.0)
            no_amount = filled_by_outcome.get("NO", 0.0)

            # Calculate how many complete sets we can merge
            mergeable_amount = min(yes_amount, no_amount)
            merge_amount = max(0.0, mergeable_amount - already_merged_amount)

            if merge_amount > 0:
                logger.info(
                    f"Found {merge_amount} new mergeable sets for {market.market_slug} "
                    f"(total YES={yes_amount}, NO={no_amount}, merged={already_merged_amount})"
                )

                # Merge the positions
                merger = CTFMerger()
                tx_hash = merger.merge_positions(market.condition_id, merge_amount)

                if tx_hash:
                    logger.info(f"Merged {merge_amount} sets -> {merge_amount} USDC")
                    return merge_amount
                else:
                    logger.error("Merge failed")
                    return 0.0
            else:
                logger.info(f"No new mergeable positions for {market.market_slug}")
                return 0.0

        except Exception as e:
            logger.error(f"Error in merge_positions_if_possible: {e}", exc_info=True)
            return 0.0
