"""Main bot logic for Polymarket limit order automation."""

import time
import threading
import json
import os
from datetime import datetime, timedelta
from typing import List, Dict
from models import Market, OrderRecord, OrderStatus, BotState, OrderSide
from market_discovery import MarketDiscovery
from order_manager import OrderManager
from auto_redeem import AutoRedeemer
from logger import logger
from config import Config


class PolymarketBot:
    """Automated limit order bot for BTC 15-minute markets."""

    def __init__(self):
        """Initialize the bot."""
        self.discovery = MarketDiscovery()
        self.order_manager = OrderManager(Config.PRIVATE_KEY)
        self.auto_redeemer = AutoRedeemer()

        # State tracking
        self.state = BotState()
        self.state.is_running = False

        # Tracked markets and orders
        self.tracked_markets: Dict[str, Market] = {}  # condition_id -> Market
        self.orders_placed: Dict[str, bool] = {}  # condition_id -> placed flag
        self.active_orders: Dict[str, List[OrderRecord]] = {}  # condition_id -> orders
        self.positions_sold: Dict[str, bool] = {}  # condition_id -> finalization flag
        self.last_merge_attempt: Dict[str, datetime] = {}  # condition_id -> timestamp
        self.merged_amounts: Dict[str, float] = {}  # condition_id -> total merged sets

        # Redemption tracking
        self.last_redemption_check = None

        # Order persistence file
        self.orders_file = "bot_orders.json"
        self.order_history_file = "order_history.json"
        self.order_history: Dict[str, OrderRecord] = {}

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
            # Initialize balance immediately so dashboard shows correct data
            self.state.usdc_balance = self.order_manager.get_usdc_balance()
            logger.info(f"Initial USDC balance: ${self.state.usdc_balance:.2f}")

        # Load persisted orders from file
        self._load_order_history()
        self._load_orders_from_file()

        # Recover existing orders from orderbook
        self._recover_existing_orders()

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

            # Step 0: Check for redeemable positions
            if (self.last_redemption_check is None or
                datetime.now() - self.last_redemption_check >
                timedelta(seconds=Config.REDEEM_CHECK_INTERVAL_SECONDS)):

                logger.info("Checking for redeemable positions...")
                redeemed_count = self.auto_redeemer.check_and_redeem_all()

                if redeemed_count > 0:
                    logger.info(f"✓ Claimed winnings from {redeemed_count} resolved markets")

                self.last_redemption_check = datetime.now()

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

            # Step 4.5: Place fallback orders if idle and no positions pending
            self._place_fallback_orders_if_idle(upcoming_markets)

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

                # Save orders to file for persistence
                self._save_orders_to_file()
                for order in orders:
                    self._upsert_order_history(order)

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
        status_changed = False

        for condition_id, orders in list(self.active_orders.items()):
            market = self.tracked_markets.get(condition_id)
            if not market:
                if self._refresh_orphaned_orders(condition_id, orders):
                    status_changed = True
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
                        status_changed = True
                    self._upsert_order_history(updated_order)
                else:
                    self._upsert_order_history(order)

            now = datetime.now()

            # Attempt merges every 30 seconds while market is active
            if not self.positions_sold.get(condition_id, False):
                last_attempt = self.last_merge_attempt.get(condition_id)
                if last_attempt is None or (now - last_attempt).total_seconds() >= 30:
                    merged_amount = self.order_manager.merge_positions_if_possible(
                        market,
                        orders,
                        already_merged_amount=self.merged_amounts.get(condition_id, 0.0)
                    )
                    if merged_amount > 0:
                        self.merged_amounts[condition_id] = (
                            self.merged_amounts.get(condition_id, 0.0) + merged_amount
                        )
                    self.last_merge_attempt[condition_id] = now
                    if self._all_positions_merged(orders, self.merged_amounts.get(condition_id, 0.0)):
                        self.positions_sold[condition_id] = True
                        status_changed = True
                        self._save_orders_to_file()

            # Sell any one-sided positions 1 minute before market end
            if (not self.positions_sold.get(condition_id, False) and
                now.timestamp() >= market.end_timestamp - 60):
                unfilled = [
                    o for o in orders
                    if o.status in [OrderStatus.PLACED, OrderStatus.PARTIALLY_FILLED]
                ]
                if unfilled:
                    logger.info(
                        f"1 minute before end for {market.market_slug}. "
                        f"Cancelling {len(unfilled)} unfilled orders."
                    )
                    cancelled_count = self.order_manager.cancel_orders(unfilled)
                    logger.info(f"Cancelled {cancelled_count} orders")
                    if cancelled_count > 0:
                        status_changed = True

                self._sell_remaining_positions(
                    market,
                    orders,
                    self.merged_amounts.get(condition_id, 0.0)
                )
                self.positions_sold[condition_id] = True

                # Save updated order state
                self._save_orders_to_file()

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
                    cancelled_count = self.order_manager.cancel_orders(unfilled)
                    if cancelled_count > 0:
                        status_changed = True

        # Save to file if any status changed
        if status_changed:
            self._save_orders_to_file()

    def _refresh_orphaned_orders(self, condition_id: str, orders: List[OrderRecord]) -> bool:
        """Refresh order statuses even if the market is no longer tracked."""
        updated_orders = []
        changed = False

        for order in orders:
            if order.status in [OrderStatus.PLACED, OrderStatus.PARTIALLY_FILLED]:
                try:
                    updated_order = self.order_manager.check_order_status(order)
                    if updated_order.status != order.status:
                        changed = True
                    order = updated_order
                except Exception as e:
                    logger.warning(f"Could not refresh order {order.order_id}: {e}")

            self._upsert_order_history(order)

            if order.status in [OrderStatus.PLACED, OrderStatus.PARTIALLY_FILLED, OrderStatus.FILLED]:
                updated_orders.append(order)
            else:
                changed = True

        if updated_orders:
            self.active_orders[condition_id] = updated_orders
            if not self.positions_sold.get(condition_id, False):
                now = datetime.now()
                last_attempt = self.last_merge_attempt.get(condition_id)
                if last_attempt is None or (now - last_attempt).total_seconds() >= 30:
                    market_stub = self._build_orphan_market(condition_id, updated_orders)
                    merged_amount = self.order_manager.merge_positions_if_possible(
                        market_stub,
                        updated_orders,
                        already_merged_amount=self.merged_amounts.get(condition_id, 0.0)
                    )
                    if merged_amount > 0:
                        self.merged_amounts[condition_id] = (
                            self.merged_amounts.get(condition_id, 0.0) + merged_amount
                        )
                        changed = True
                    self.last_merge_attempt[condition_id] = now
                    if self._all_positions_merged(
                        updated_orders,
                        self.merged_amounts.get(condition_id, 0.0)
                    ):
                        self.positions_sold[condition_id] = True
                        changed = True
            return changed

        logger.info(
            f"Orphaned orders cleared for {condition_id}; "
            "no live orders remain"
        )
        self.active_orders.pop(condition_id, None)
        self.orders_placed.pop(condition_id, None)
        self.positions_sold.pop(condition_id, None)
        self.last_merge_attempt.pop(condition_id, None)
        self.merged_amounts.pop(condition_id, None)
        return True

    def _build_orphan_market(self, condition_id: str, orders: List[OrderRecord]) -> Market:
        """Create a minimal market object for orphaned orders."""
        now_ts = int(datetime.now().timestamp())
        market_slug = orders[0].market_slug if orders else f"orphaned-{condition_id[:16]}"
        return Market(
            condition_id=condition_id,
            market_slug=market_slug,
            question="Orphaned market",
            start_timestamp=now_ts - 60,
            end_timestamp=now_ts + 3600,
            outcomes=[]
        )

    def _place_fallback_orders_if_idle(self, upcoming_markets: List[Market]):
        """Place orders on the next upcoming market if the bot is idle."""
        if not upcoming_markets:
            return

        has_live_orders = False
        for orders in self.active_orders.values():
            if any(o.status in [OrderStatus.PLACED, OrderStatus.PARTIALLY_FILLED] for o in orders):
                has_live_orders = True
                break

        if has_live_orders:
            return

        has_unprocessed_positions = False
        for condition_id, orders in self.active_orders.items():
            if self.positions_sold.get(condition_id, False):
                continue
            if any(o.status in [OrderStatus.FILLED, OrderStatus.PARTIALLY_FILLED] for o in orders):
                has_unprocessed_positions = True
                break

        if has_unprocessed_positions:
            return

        now_ts = datetime.now().timestamp()
        future_markets = [m for m in upcoming_markets if m.start_timestamp > now_ts]
        if not future_markets:
            return

        next_market = min(future_markets, key=lambda m: m.start_timestamp)
        if self.orders_placed.get(next_market.condition_id, False):
            return

        logger.info(
            f"Idle state detected. Placing fallback orders for next market: "
            f"{next_market.market_slug} (starts in {next_market.time_until_start/60:.1f} minutes)"
        )

        try:
            orders = self.order_manager.place_simple_test_orders(
                market=next_market,
                price=0.49,
                size=10.0
            )

            if orders:
                self.orders_placed[next_market.condition_id] = True
                self.active_orders[next_market.condition_id] = orders
                self._save_orders_to_file()
                for order in orders:
                    self._upsert_order_history(order)

                logger.info(
                    f"Successfully placed {len(orders)} fallback orders for "
                    f"{next_market.market_slug}"
                )
                for order in orders:
                    logger.info(
                        f"  - {order.side.value} {order.outcome} @ "
                        f"${order.price:.2f} x {order.size:.2f} shares"
                    )
            else:
                logger.error(f"Failed to place fallback orders for {next_market.market_slug}")

        except Exception as e:
            logger.error(
                f"Error placing fallback orders for {next_market.market_slug}: {e}",
                exc_info=True
            )

    def _normalize_outcome(self, outcome: str) -> str:
        """Normalize outcome names for YES/NO classification."""
        outcome_upper = outcome.strip().upper()
        if outcome_upper in ["YES", "UP"]:
            return "YES"
        if outcome_upper in ["NO", "DOWN"]:
            return "NO"
        return ""

    def _get_filled_amounts(self, orders: List[OrderRecord]) -> Dict[str, float]:
        """Get total filled amounts per outcome (YES/NO)."""
        filled = {"YES": 0.0, "NO": 0.0}
        for order in orders:
            if order.status in [OrderStatus.FILLED, OrderStatus.PARTIALLY_FILLED]:
                try:
                    order_details = self.order_manager.client.get_order(order.order_id)
                    size_matched = float(order_details.get("size_matched", 0))
                    if size_matched > 0:
                        normalized = self._normalize_outcome(order.outcome)
                        if normalized:
                            filled[normalized] += size_matched
                except Exception as e:
                    logger.warning(f"Could not get filled size for order {order.order_id}: {e}")
        return filled

    def _all_positions_merged(self, orders: List[OrderRecord], merged_amount: float) -> bool:
        """Return True if all filled positions have been merged."""
        filled_amounts = self._get_filled_amounts(orders)
        remaining_yes = filled_amounts["YES"] - merged_amount
        remaining_no = filled_amounts["NO"] - merged_amount

        has_live_orders = any(
            o.status in [OrderStatus.PLACED, OrderStatus.PARTIALLY_FILLED]
            for o in orders
        )

        return (
            remaining_yes <= 0.0
            and remaining_no <= 0.0
            and not has_live_orders
        )

    def _sell_remaining_positions(
        self,
        market: Market,
        orders: List[OrderRecord],
        merged_amount: float
    ):
        """
        Sell any remaining one-sided positions at market price.

        Args:
            market: Market to sell positions in
            orders: List of orders that were placed
            merged_amount: Total amount already merged
        """
        try:
            filled_amounts = self._get_filled_amounts(orders)
            remaining_yes = max(0.0, filled_amounts["YES"] - merged_amount)
            remaining_no = max(0.0, filled_amounts["NO"] - merged_amount)

            if remaining_yes <= 0 and remaining_no <= 0:
                logger.info(f"No filled positions to sell for {market.market_slug}")
                return

            logger.info(
                f"Selling remaining one-sided positions for {market.market_slug} "
                f"(YES={remaining_yes:.2f}, NO={remaining_no:.2f})"
            )

            yes_outcome = next(
                (o for o in market.outcomes if self._normalize_outcome(o.outcome) == "YES"),
                None
            )
            no_outcome = next(
                (o for o in market.outcomes if self._normalize_outcome(o.outcome) == "NO"),
                None
            )

            if remaining_yes > 0 and yes_outcome:
                sell_order = self.order_manager.sell_position_market(
                    market=market,
                    outcome=yes_outcome,
                    size=remaining_yes
                )
                if sell_order:
                    logger.info(
                        f"Successfully placed sell order {sell_order.order_id} "
                        f"for {remaining_yes:.2f} shares at ${sell_order.price:.2f}"
                    )
                else:
                    logger.error("Failed to sell YES/UP position")
                time.sleep(0.5)

            if remaining_no > 0 and no_outcome:
                sell_order = self.order_manager.sell_position_market(
                    market=market,
                    outcome=no_outcome,
                    size=remaining_no
                )
                if sell_order:
                    logger.info(
                        f"Successfully placed sell order {sell_order.order_id} "
                        f"for {remaining_no:.2f} shares at ${sell_order.price:.2f}"
                    )
                else:
                    logger.error("Failed to sell NO/DOWN position")
                time.sleep(0.5)

        except Exception as e:
            logger.error(f"Error in _sell_remaining_positions: {e}", exc_info=True)

    def _upsert_order_history(self, order: OrderRecord):
        """Insert or update an order in history."""
        self.order_history[order.order_id] = order

    def _sync_history_from_active_orders(self):
        """Sync active orders into history."""
        for orders in self.active_orders.values():
            for order in orders:
                self._upsert_order_history(order)

    def _load_order_history(self):
        """Load order history from file for dashboard display."""
        try:
            if not os.path.exists(self.order_history_file):
                return

            with open(self.order_history_file, "r") as f:
                history_data = json.load(f)

            if not isinstance(history_data, list):
                return

            for order_dict in history_data:
                try:
                    order = OrderRecord(
                        order_id=order_dict["order_id"],
                        market_slug=order_dict["market_slug"],
                        condition_id=order_dict["condition_id"],
                        token_id=order_dict["token_id"],
                        outcome=order_dict["outcome"],
                        side=OrderSide(order_dict["side"]),
                        price=order_dict["price"],
                        size=order_dict["size"],
                        size_usd=order_dict["size_usd"],
                        status=OrderStatus(order_dict["status"]),
                        created_at=datetime.fromisoformat(order_dict["created_at"]),
                        filled_at=datetime.fromisoformat(order_dict["filled_at"]) if order_dict.get("filled_at") else None,
                        error_message=order_dict.get("error_message")
                    )
                    self._upsert_order_history(order)
                except Exception as e:
                    logger.warning(f"Could not load history order {order_dict.get('order_id', 'unknown')}: {e}")

            logger.info(f"Loaded {len(self.order_history)} orders from {self.order_history_file}")

        except Exception as e:
            logger.error(f"Error loading order history: {e}", exc_info=True)

    def _save_order_history(self):
        """Save order history to file."""
        try:
            history_list = []
            for order in self.order_history.values():
                history_list.append({
                    "order_id": order.order_id,
                    "market_slug": order.market_slug,
                    "condition_id": order.condition_id,
                    "token_id": order.token_id,
                    "outcome": order.outcome,
                    "side": order.side.value,
                    "price": order.price,
                    "size": order.size,
                    "size_usd": order.size_usd,
                    "status": order.status.value,
                    "created_at": order.created_at.isoformat() if order.created_at else None,
                    "filled_at": order.filled_at.isoformat() if order.filled_at else None,
                    "error_message": order.error_message
                })

            history_list.sort(key=lambda o: o.get("created_at") or "", reverse=True)
            with open(self.order_history_file, "w") as f:
                json.dump(history_list, f, indent=2)

        except Exception as e:
            logger.error(f"Error saving order history: {e}", exc_info=True)

    def _recover_existing_orders(self):
        """Recover existing orders from orderbook on startup."""
        try:
            logger.info("Recovering existing orders from orderbook...")
            from py_clob_client.client import OpenOrderParams

            # Get all open orders for this wallet
            try:
                all_open_orders = self.order_manager.client.get_orders(OpenOrderParams())
            except Exception as e:
                logger.warning(f"Could not fetch orders: {e}")
                return

            if not all_open_orders:
                logger.info("No existing orders found in orderbook")
                return

            # Convert to OrderRecord objects and add to tracking
            from models import OrderRecord, OrderStatus, OrderSide
            recovered_count = 0

            for order_data in all_open_orders:
                try:
                    # Extract order details
                    order_id = order_data.get('id', '')
                    market_condition = order_data.get('market', '')
                    token_id = order_data.get('asset_id', '')
                    price = float(order_data.get('price', 0))
                    size = float(order_data.get('size', 0))
                    side = OrderSide.BUY if order_data.get('side', '') == 'BUY' else OrderSide.SELL

                    # Create OrderRecord
                    order_record = OrderRecord(
                        order_id=order_id,
                        market_slug=f"recovered-{market_condition[:16]}",
                        condition_id=market_condition,
                        token_id=token_id,
                        outcome="Unknown",  # Will be updated later
                        side=side,
                        price=price,
                        size=size,
                        size_usd=price * size,
                        status=OrderStatus.PLACED,
                        created_at=datetime.now()
                    )

                    # Add to active_orders
                    if market_condition not in self.active_orders:
                        self.active_orders[market_condition] = []
                    self.active_orders[market_condition].append(order_record)

                    # Mark market as having orders placed
                    self.orders_placed[market_condition] = True

                    recovered_count += 1

                except Exception as e:
                    logger.warning(f"Could not recover order {order_data.get('id', 'unknown')}: {e}")

            logger.info(f"Recovered {recovered_count} existing orders from orderbook")

            # Update order lists for dashboard
            with self.lock:
                self._update_order_lists()

        except Exception as e:
            logger.error(f"Error recovering existing orders: {e}", exc_info=True)

    def _save_orders_to_file(self):
        """Save current orders to file for persistence across restarts."""
        try:
            # Convert orders to serializable format
            orders_data = {}
            for condition_id, orders in self.active_orders.items():
                orders_data[condition_id] = [
                    {
                        "order_id": o.order_id,
                        "market_slug": o.market_slug,
                        "condition_id": o.condition_id,
                        "token_id": o.token_id,
                        "outcome": o.outcome,
                        "side": o.side.value,
                        "price": o.price,
                        "size": o.size,
                        "size_usd": o.size_usd,
                        "status": o.status.value,
                        "created_at": o.created_at.isoformat(),
                        "error_message": o.error_message
                    }
                    for o in orders
                ]

            # Save to file
            with open(self.orders_file, 'w') as f:
                json.dump(orders_data, f, indent=2)

            self._sync_history_from_active_orders()
            self._save_order_history()

            logger.debug(f"Saved {sum(len(orders) for orders in self.active_orders.values())} orders to {self.orders_file}")

        except Exception as e:
            logger.error(f"Error saving orders to file: {e}", exc_info=True)

    def _load_orders_from_file(self):
        """Load orders from file on startup."""
        try:
            if not os.path.exists(self.orders_file):
                logger.info("No persisted orders file found")
                return

            with open(self.orders_file, 'r') as f:
                orders_data = json.load(f)

            if not orders_data:
                logger.info("No persisted orders to load")
                return

            # Convert back to OrderRecord objects
            from models import OrderRecord, OrderStatus, OrderSide

            loaded_count = 0
            for condition_id, orders_list in orders_data.items():
                self.active_orders[condition_id] = []
                for order_dict in orders_list:
                    try:
                        order = OrderRecord(
                            order_id=order_dict["order_id"],
                            market_slug=order_dict["market_slug"],
                            condition_id=order_dict["condition_id"],
                            token_id=order_dict["token_id"],
                            outcome=order_dict["outcome"],
                            side=OrderSide(order_dict["side"]),
                            price=order_dict["price"],
                            size=order_dict["size"],
                            size_usd=order_dict["size_usd"],
                            status=OrderStatus(order_dict["status"]),
                            created_at=datetime.fromisoformat(order_dict["created_at"]),
                            error_message=order_dict.get("error_message")
                        )
                        self.active_orders[condition_id].append(order)
                        self.orders_placed[condition_id] = True
                        loaded_count += 1
                    except Exception as e:
                        logger.warning(f"Could not load order {order_dict.get('order_id', 'unknown')}: {e}")

            logger.info(f"Loaded {loaded_count} orders from {self.orders_file}")

            # Check for old orders that aren't in tracked markets and finalize their statuses
            self._finalize_orphaned_orders()

            # Update order lists for dashboard
            with self.lock:
                self._update_order_lists()

        except Exception as e:
            logger.error(f"Error loading orders from file: {e}", exc_info=True)

    def _cleanup_old_markets(self):
        """Remove old markets and update order statuses."""
        cutoff = datetime.now().timestamp() - 86400  # 24 hours ago

        # Find old markets
        old_conditions = [
            cid for cid, market in self.tracked_markets.items()
            if market.end_timestamp < cutoff
        ]

        if not old_conditions:
            return

        logger.info(f"Cleaning up {len(old_conditions)} old markets and updating order statuses")

        for condition_id in old_conditions:
            # Update order statuses before cleanup
            if condition_id in self.active_orders:
                self._finalize_old_order_statuses(condition_id)

            # Remove from tracking
            logger.debug(f"Cleaning up old market: {condition_id}")
            self.tracked_markets.pop(condition_id, None)
            self.orders_placed.pop(condition_id, None)
            self.active_orders.pop(condition_id, None)
            self.positions_sold.pop(condition_id, None)
            self.last_merge_attempt.pop(condition_id, None)
            self.merged_amounts.pop(condition_id, None)

        # Save updated statuses to file
        self._save_orders_to_file()

    def _finalize_old_order_statuses(self, condition_id: str):
        """
        Finalize order statuses for an old market before cleanup.

        Args:
            condition_id: Market condition ID
        """
        try:
            orders = self.active_orders.get(condition_id, [])
            if not orders:
                return

            market_slug = orders[0].market_slug if orders else "unknown"
            logger.info(f"Finalizing order statuses for old market: {market_slug}")

            for order in orders:
                # Skip already-finalized statuses
                if order.status in [OrderStatus.FILLED, OrderStatus.CANCELLED, OrderStatus.FAILED]:
                    continue

                # Check current status from orderbook
                try:
                    updated_order = self.order_manager.check_order_status(order)

                    # If still showing as PLACED/PARTIALLY_FILLED after market is old,
                    # it was likely cancelled or expired
                    if updated_order.status in [OrderStatus.PLACED, OrderStatus.PARTIALLY_FILLED]:
                        logger.info(
                            f"Order {order.order_id[:16]}... still shows as {order.status.value} "
                            f"for old market - marking as CANCELLED"
                        )
                        order.status = OrderStatus.CANCELLED

                except Exception as e:
                    logger.warning(f"Could not check final status for order {order.order_id}: {e}")
                    # If we can't check, assume it's cancelled (market is >24h old)
                    if order.status in [OrderStatus.PLACED, OrderStatus.PARTIALLY_FILLED]:
                        order.status = OrderStatus.CANCELLED

            logger.info(f"Finalized {len(orders)} orders for {market_slug}")

        except Exception as e:
            logger.error(f"Error finalizing order statuses: {e}", exc_info=True)

    def _finalize_orphaned_orders(self):
        """
        Finalize order statuses for loaded orders that don't have a tracked market.
        This handles orders loaded from file on startup for markets that have ended.
        """
        try:
            orphaned_conditions = [
                cid for cid in self.active_orders.keys()
                if cid not in self.tracked_markets
            ]

            if not orphaned_conditions:
                return

            logger.info(f"Found {len(orphaned_conditions)} orphaned order groups - checking statuses")
            status_changed = False

            for condition_id in orphaned_conditions:
                orders = self.active_orders.get(condition_id, [])
                if not orders:
                    continue

                market_slug = orders[0].market_slug if orders else "unknown"
                logger.info(f"Checking orphaned orders for market: {market_slug}")

                for order in orders:
                    # Skip already-finalized statuses
                    if order.status in [OrderStatus.FILLED, OrderStatus.CANCELLED, OrderStatus.FAILED]:
                        continue

                    # Check current status from orderbook
                    try:
                        updated_order = self.order_manager.check_order_status(order)

                        # Log status changes
                        if updated_order.status != order.status:
                            logger.info(
                                f"Order {order.order_id[:16]}... status updated: "
                                f"{order.status.value} -> {updated_order.status.value}"
                            )
                            status_changed = True

                        # If still showing as PLACED/PARTIALLY_FILLED but market is orphaned,
                        # it was likely cancelled or expired
                        if updated_order.status in [OrderStatus.PLACED, OrderStatus.PARTIALLY_FILLED]:
                            logger.info(
                                f"Order {order.order_id[:16]}... still shows as {order.status.value} "
                                f"for orphaned market - marking as CANCELLED"
                            )
                            order.status = OrderStatus.CANCELLED
                            status_changed = True

                    except Exception as e:
                        logger.warning(f"Could not check status for orphaned order {order.order_id}: {e}")
                        # If we can't check and it's not already finalized, mark as cancelled
                        if order.status in [OrderStatus.PLACED, OrderStatus.PARTIALLY_FILLED]:
                            logger.info(f"Marking unreachable orphaned order as CANCELLED")
                            order.status = OrderStatus.CANCELLED
                            status_changed = True

            # Save if any statuses changed
            if status_changed:
                self._save_orders_to_file()
                logger.info("Saved updated orphaned order statuses to file")

        except Exception as e:
            logger.error(f"Error finalizing orphaned orders: {e}", exc_info=True)

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
        history_orders = list(self.order_history.values())
        history_orders.sort(
            key=lambda o: o.created_at if o.created_at else datetime.min,
            reverse=True
        )
        recent = history_orders[:100]  # Last 100 orders

        self.state.pending_orders = pending
        self.state.recent_orders = recent

    def get_state(self) -> BotState:
        """Get current bot state (thread-safe)."""
        with self.lock:
            return self.state.model_copy(deep=True)


# Global bot instance for dashboard access
_bot_instance = None
_bot_lock = threading.Lock()


def get_bot_instance() -> PolymarketBot:
    """Get or create global bot instance (thread-safe singleton)."""
    global _bot_instance
    if _bot_instance is None:
        with _bot_lock:
            # Double-check locking pattern
            if _bot_instance is None:
                _bot_instance = PolymarketBot()
                logger.info(f"Created new bot instance: {id(_bot_instance)}")
    return _bot_instance


def reset_bot_instance():
    """Reset global bot instance (for testing/debugging)."""
    global _bot_instance
    with _bot_lock:
        if _bot_instance is not None:
            logger.info(f"Resetting bot instance: {id(_bot_instance)}")
            _bot_instance = None


def run_bot_loop():
    """Main bot loop that runs continuously."""
    bot = get_bot_instance()  # Use global instance!
    logger.info(f"Bot loop: bot instance id={id(bot)}")
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
