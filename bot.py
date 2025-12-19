"""Main bot logic for Polymarket limit order automation."""

import time
import threading
from datetime import datetime, timedelta
from typing import List, Dict
from models import Market, OrderRecord, OrderStatus, BotState
from market_discovery import MarketDiscovery
from order_manager import OrderManager
from logger import logger
from config import Config


class PolymarketBot:
    """Automated limit order bot for BTC 15-minute markets."""

    def __init__(self):
        """Initialize the bot."""
        self.discovery = MarketDiscovery()
        self.order_manager = OrderManager(Config.PRIVATE_KEY)

        # State tracking
        self.state = BotState()
        self.state.is_running = False

        # Tracked markets and orders
        self.tracked_markets: Dict[str, Market] = {}  # condition_id -> Market
        self.orders_placed: Dict[str, bool] = {}  # condition_id -> placed flag
        self.active_orders: Dict[str, List[OrderRecord]] = {}  # condition_id -> orders
        self.positions_sold: Dict[str, bool] = {}  # condition_id -> sold flag

        # Lock for thread safety
        self.lock = threading.Lock()

    def start(self):
        """Start the bot."""
        logger.info("=" * 60)
        logger.info("Starting Polymarket Limit Order Bot")
        logger.info("=" * 60)
        logger.info(f"Wallet address: {self.order_manager.address}")
        logger.info(f"Order size: ${Config.ORDER_SIZE_USD} per order")
        logger.info(f"Spread offset: {Config.SPREAD_OFFSET}")
        logger.info(f"Order placement: {Config.ORDER_PLACEMENT_MINUTES_BEFORE} min before start")
        logger.info("=" * 60)

        with self.lock:
            self.state.is_running = True

    def stop(self):
        """Stop the bot."""
        logger.info("Stopping bot...")
        with self.lock:
            self.state.is_running = False

    def run_once(self):
        """Run one iteration of the bot loop."""
        try:
            with self.lock:
                self.state.last_check = datetime.now()

            # Step 1: Discover markets
            logger.info("Discovering BTC 15-minute markets...")
            markets = self.discovery.discover_btc_15m_markets()

            # Step 2: Filter for upcoming/active markets
            upcoming_markets = self._filter_upcoming_markets(markets)

            with self.lock:
                self.state.active_markets = upcoming_markets

            logger.info(f"Found {len(upcoming_markets)} upcoming/active markets")

            # Step 3: Check each market for order placement timing
            for market in upcoming_markets:
                self._process_market(market)

            # Step 4: Check status of active orders
            self._check_active_orders()

            # Step 5: Clean up old markets and orders
            self._cleanup_old_markets()

            # Step 6: Update state
            with self.lock:
                self.state.usdc_balance = self.order_manager.get_usdc_balance()
                self._update_order_lists()

        except Exception as e:
            logger.error(f"Error in bot loop: {e}", exc_info=True)
            with self.lock:
                self.state.error_count += 1
                self.state.last_error = str(e)

    def _filter_upcoming_markets(self, markets: List[Market]) -> List[Market]:
        """Filter for upcoming and active markets (not yet ended)."""
        upcoming = []
        now = datetime.now().timestamp()

        for market in markets:
            # Skip resolved markets
            if market.is_resolved:
                continue

            # Include markets that haven't ended yet (within next 24 hours OR currently active)
            # A market is relevant if:
            # 1. It starts in the next 24 hours, OR
            # 2. It has started but not ended yet
            time_until_start = market.start_timestamp - now
            time_until_end = market.end_timestamp - now

            # Include if starting soon OR currently running
            if time_until_end > -300:  # Haven't ended (allow 5 min grace period after end)
                if time_until_start <= 86400:  # Starts within 24 hours or already started
                    upcoming.append(market)

                # Add to tracked markets
                if market.condition_id not in self.tracked_markets:
                    self.tracked_markets[market.condition_id] = market
                    self.orders_placed[market.condition_id] = False
                    logger.info(
                        f"Tracking new market: {market.market_slug} "
                        f"(starts in {time_until_start/60:.1f} minutes)"
                    )

        return upcoming

    def _process_market(self, market: Market):
        """Process a single market for order placement."""
        condition_id = market.condition_id

        # Check if we already placed orders
        if self.orders_placed.get(condition_id, False):
            return

        # Check if it's time to place orders
        if not market.should_place_orders:
            time_until_start = market.time_until_start
            if time_until_start > 0:
                logger.debug(
                    f"Market {market.market_slug}: {time_until_start/60:.1f} min "
                    f"until placement time"
                )
            return

        # Time to place orders!
        logger.info(
            f"Placing orders for {market.market_slug} "
            f"(starts in {market.time_until_start/60:.1f} minutes)"
        )

        try:
            # Place simple test orders: 2 orders (1 Yes, 1 No) at $0.49, 10 shares each
            orders = self.order_manager.place_simple_test_orders(
                market=market,
                price=0.49,
                size=10.0
            )

            if orders:
                # Mark as placed
                self.orders_placed[condition_id] = True
                self.active_orders[condition_id] = orders

                logger.info(
                    f"Successfully placed {len(orders)} orders for "
                    f"{market.market_slug}"
                )

                # Log order details
                for order in orders:
                    logger.info(
                        f"  - {order.side.value} {order.outcome} @ "
                        f"${order.price:.2f} x {order.size:.2f} shares"
                    )
            else:
                logger.error(f"Failed to place orders for {market.market_slug}")

        except Exception as e:
            logger.error(
                f"Error processing market {market.market_slug}: {e}",
                exc_info=True
            )

    def _check_active_orders(self):
        """Check status of all active orders."""
        for condition_id, orders in list(self.active_orders.items()):
            market = self.tracked_markets.get(condition_id)
            if not market:
                continue

            # Skip if market is too old
            if market.end_timestamp < (datetime.now().timestamp() - 3600):
                continue

            # Check each order
            for order in orders:
                if order.status in [OrderStatus.PLACED, OrderStatus.PARTIALLY_FILLED]:
                    updated_order = self.order_manager.check_order_status(order)

                    # Log status changes
                    if updated_order.status != order.status:
                        logger.info(
                            f"Order {order.order_id} status changed: "
                            f"{order.status.value} -> {updated_order.status.value}"
                        )

            # Check if 10 minutes have passed since order placement
            if orders and orders[0].created_at and not self.positions_sold.get(condition_id, False):
                time_since_placement = (datetime.now() - orders[0].created_at).total_seconds()

                # After 10 minutes, cancel unfilled orders and sell positions
                if time_since_placement >= 600:  # 10 minutes
                    unfilled = [
                        o for o in orders
                        if o.status in [OrderStatus.PLACED, OrderStatus.PARTIALLY_FILLED]
                    ]

                    logger.info(
                        f"10 minutes elapsed for {market.market_slug}. "
                        f"Cancelling {len(unfilled)} unfilled orders and selling positions."
                    )

                    # Cancel unfilled orders
                    if unfilled:
                        cancelled_count = self.order_manager.cancel_orders(unfilled)
                        logger.info(f"Cancelled {cancelled_count} orders")

                    # Sell any filled positions at market price
                    self._sell_positions(market, orders)

                    # Mark this market as processed (won't sell again)
                    self.positions_sold[condition_id] = True

            # Also cancel unfilled orders after market ends
            if datetime.now().timestamp() > market.end_timestamp + 300:  # 5 min grace
                unfilled = [
                    o for o in orders
                    if o.status in [OrderStatus.PLACED, OrderStatus.PARTIALLY_FILLED]
                ]
                if unfilled:
                    logger.info(
                        f"Market ended. Cancelling {len(unfilled)} unfilled orders for "
                        f"{market.market_slug}"
                    )
                    self.order_manager.cancel_orders(unfilled)

    def _sell_positions(self, market: Market, orders: List[OrderRecord]):
        """
        Sell any filled positions at market price.

        Args:
            market: Market to sell positions in
            orders: List of orders that were placed
        """
        try:
            # Check which orders were filled or partially filled
            filled_orders = [
                o for o in orders
                if o.status in [OrderStatus.FILLED, OrderStatus.PARTIALLY_FILLED]
            ]

            if not filled_orders:
                logger.info(f"No filled positions to sell for {market.market_slug}")
                return

            logger.info(
                f"Selling {len(filled_orders)} filled positions for {market.market_slug}"
            )

            # For each filled order, check the latest status and sell the position
            for order in filled_orders:
                # Get latest order details to find filled size
                try:
                    order_details = self.order_manager.client.get_order(order.order_id)
                    size_matched = float(order_details.get("size_matched", 0))

                    if size_matched > 0:
                        # Find the outcome
                        outcome = None
                        for o in market.outcomes:
                            if o.token_id == order.token_id:
                                outcome = o
                                break

                        if outcome:
                            logger.info(
                                f"Selling {size_matched:.2f} shares of {outcome.outcome} "
                                f"from order {order.order_id}"
                            )

                            # Sell at market price
                            sell_order = self.order_manager.sell_position_market(
                                market=market,
                                outcome=outcome,
                                size=size_matched
                            )

                            if sell_order:
                                logger.info(
                                    f"Successfully placed sell order {sell_order.order_id} "
                                    f"for {size_matched:.2f} shares at ${sell_order.price:.2f}"
                                )
                            else:
                                logger.error(f"Failed to sell position for {outcome.outcome}")

                            time.sleep(0.5)  # Rate limiting

                except Exception as e:
                    logger.error(f"Error selling position for order {order.order_id}: {e}")

        except Exception as e:
            logger.error(f"Error in _sell_positions: {e}", exc_info=True)

    def _cleanup_old_markets(self):
        """Remove old markets and orders from tracking."""
        cutoff = datetime.now().timestamp() - 86400  # 24 hours ago

        # Clean up tracked markets
        old_conditions = [
            cid for cid, market in self.tracked_markets.items()
            if market.end_timestamp < cutoff
        ]

        for condition_id in old_conditions:
            logger.debug(f"Cleaning up old market: {condition_id}")
            self.tracked_markets.pop(condition_id, None)
            self.orders_placed.pop(condition_id, None)
            self.active_orders.pop(condition_id, None)
            self.positions_sold.pop(condition_id, None)

    def _update_order_lists(self):
        """Update order lists in state for dashboard."""
        all_orders = []
        for orders in self.active_orders.values():
            all_orders.extend(orders)

        # Sort by creation time
        all_orders.sort(key=lambda o: o.created_at, reverse=True)

        # Separate pending and recent
        pending = [
            o for o in all_orders
            if o.status in [OrderStatus.PLACED, OrderStatus.PARTIALLY_FILLED]
        ]
        recent = all_orders[:50]  # Last 50 orders

        self.state.pending_orders = pending
        self.state.recent_orders = recent

    def get_state(self) -> BotState:
        """Get current bot state (thread-safe)."""
        with self.lock:
            return self.state.model_copy(deep=True)


# Global bot instance for dashboard access
_bot_instance = None


def get_bot_instance() -> PolymarketBot:
    """Get or create global bot instance."""
    global _bot_instance
    if _bot_instance is None:
        _bot_instance = PolymarketBot()
    return _bot_instance


def run_bot_loop():
    """Main bot loop that runs continuously."""
    bot = get_bot_instance()  # Use global instance!
    bot.start()

    try:
        while bot.state.is_running:
            bot.run_once()

            # Sleep for check interval
            logger.info(
                f"Sleeping for {Config.CHECK_INTERVAL_SECONDS} seconds...\n"
            )
            time.sleep(Config.CHECK_INTERVAL_SECONDS)

    except KeyboardInterrupt:
        logger.info("Received interrupt signal")
    except Exception as e:
        logger.error(f"Fatal error in bot loop: {e}", exc_info=True)
    finally:
        bot.stop()
        logger.info("Bot stopped")
