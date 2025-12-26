"""
Split & Maker Strategy
----------------------
Strategy:
1. Split USDC into Yes + No positions (Mint).
2. Place Limit Orders (Sell) on both sides at a premium (sum > 1.0).
3. If one side fills (Legged Out), immediately check if we can sell the other side at profit.
   - If Profit: Take liquidity (Sell to Bid) on the remaining side.
   - If No Profit: Adjust Limit Order on remaining side to break-even or minimize loss.
"""

import time
import sys
from datetime import datetime
from typing import Optional, Dict

from config import Config
from logger import logger
from models import Market, OrderSide, OrderStatus
from market_discovery import MarketDiscovery
from order_manager import OrderManager
from ctf_split import CTFSplitter


class SplitMakerStrategy:
    def __init__(self, budget_usdc: float = 10.0, target_profit_pct: float = 0.01):
        self.budget = budget_usdc
        self.target_profit = target_profit_pct  # 1%
        
        self.om = OrderManager(Config.PRIVATE_KEY)
        self.splitter = CTFSplitter()
        self.discovery = MarketDiscovery()
        
        self.active_market: Optional[Market] = None
        self.positions: Dict[str, float] = {"YES": 0.0, "NO": 0.0}
        self.orders: Dict[str, str] = {}  # outcome -> order_id
        
    def run(self):
        logger.info("Starting Split & Maker Strategy...")
        
        # 1. Find suitable market
        self.active_market = self.find_next_market()
        if not self.active_market:
            logger.error("No suitable market found.")
            return

        logger.info(f"Targeting Market: {self.active_market.market_slug}")
        
        # 2. Check/Perform Split
        if not self.ensure_inventory():
            logger.error("Failed to acquire inventory.")
            return

        # 3. Main Loop: Place & Monitor Orders
        while True:
            try:
                self.cycle()
                time.sleep(2)
            except KeyboardInterrupt:
                logger.info("Stopping...")
                self.cancel_all()
                break
            except Exception as e:
                logger.error(f"Error in loop: {e}", exc_info=True)
                time.sleep(5)

    def find_next_market(self) -> Optional[Market]:
        """Find the next starting BTC market."""
        markets = self.discovery.discover_btc_15m_markets()
        now = datetime.now().timestamp()
        
        # Filter for markets starting in 5-20 mins
        candidates = []
        for m in markets:
            time_to_start = m.start_timestamp - now
            if 300 < time_to_start < 3600:  # 5 min to 1 hour
                candidates.append(m)
                
        if not candidates:
            # Fallback to nearest one
            future = [m for m in markets if m.start_timestamp > now]
            if future:
                return min(future, key=lambda m: m.start_timestamp)
        else:
            return min(candidates, key=lambda m: m.start_timestamp)
            
        return None

    def ensure_inventory(self) -> bool:
        """Check if we have positions, if not, Split USDC."""
        # Check actual balances
        # Note: This simplifies by assuming we just split if we don't have enough
        # In production, check self.om._get_token_balances()
        
        # For this demo, we assume we need to split if internal state is 0
        if self.positions["YES"] > 0 and self.positions["NO"] > 0:
            return True
            
        logger.info(f"Splitting {self.budget} USDC...")
        tx = self.splitter.split_positions(self.active_market.condition_id, self.budget)
        
        if tx:
            # Assume success and update local state (wait for confirmation in prod)
            self.positions["YES"] = self.budget
            self.positions["NO"] = self.budget
            return True
            
        return False

    def cycle(self):
        """One iteration of strategy logic."""
        market = self.om.update_market_prices(self.active_market)
        
        # Identify outcomes
        yes_outcome = next((o for o in market.outcomes if "YES" in o.outcome.upper() or "UP" in o.outcome.upper()), None)
        no_outcome = next((o for o in market.outcomes if "NO" in o.outcome.upper() or "DOWN" in o.outcome.upper()), None)
        
        if not yes_outcome or not no_outcome:
            return

        # Check existing order status
        self.check_fills(market)
        
        # Determine current holdings
        yes_held = self.positions["YES"]
        no_held = self.positions["NO"]
        
        # Case 1: Holding BOTH (Full set)
        if yes_held > 0.1 and no_held > 0.1:
            self.manage_full_set(market, yes_outcome, no_outcome)
            
        # Case 2: Holding Only YES (No sold)
        elif yes_held > 0.1:
            self.manage_leg_out(market, yes_outcome, is_yes=True)
            
        # Case 3: Holding Only NO (Yes sold)
        elif no_held > 0.1:
            self.manage_leg_out(market, no_outcome, is_yes=False)
            
        # Case 4: All Sold
        else:
            logger.info("All positions sold! Strategy Complete.")
            sys.exit(0)

    def manage_full_set(self, market, yes_outcome, no_outcome):
        """We have both. Try to sell both at premium."""
        # Calculate target prices
        # We want P_yes + P_no > 1.0 + profit
        target_total = 1.0 + self.target_profit
        
        # Look at market imbalance
        mid_yes = (yes_outcome.best_bid + yes_outcome.best_ask) / 2 if yes_outcome.best_ask else 0.5
        mid_no = (no_outcome.best_bid + no_outcome.best_ask) / 2 if no_outcome.best_ask else 0.5
        
        market_total = mid_yes + mid_no
        
        # Distribute target price based on market ratio
        # P_yes = Target * (Mid_Yes / Market_Total)
        price_yes = target_total * (mid_yes / market_total)
        price_no = target_total * (mid_no / market_total)
        
        # Round and Clamp
        price_yes = round(max(0.01, min(0.99, price_yes)), 2)
        price_no = round(max(0.01, min(0.99, price_no)), 2)
        
        # Ensure sum > 1.0 (safety)
        if price_yes + price_no < 1.01:
            price_yes += 0.01
            
        # Update Orders
        self.update_order("YES", market, yes_outcome, price_yes, self.positions["YES"])
        self.update_order("NO", market, no_outcome, price_no, self.positions["NO"])

    def manage_leg_out(self, market, outcome, is_yes: bool):
        """One side was sold. Manage the remaining side (Inventory Risk)."""
        # We sold the OTHER side.
        # Check if we can sell THIS side immediately to lock profit?
        
        # We need to know what price we sold the other side at. 
        # For simplicity, assume we sold at target/2 (approx 0.505) or track actuals.
        # Let's assume we collected ~0.505 cash already.
        cash_collected = 0.505 
        cost = 1.0
        break_even = cost - cash_collected # e.g. 0.495
        
        best_bid = outcome.best_bid
        
        if best_bid > break_even + 0.005:
            # Profit exists! Dump into bid.
            logger.info(f"Leg Out Opportunity! Selling remaining to Bid @ {best_bid}")
            self.cancel_order_for_side("YES" if is_yes else "NO")
            self.om.sell_position_market(market, outcome, self.positions["YES" if is_yes else "NO"])
            # Update local state
            self.positions["YES" if is_yes else "NO"] = 0
            
        else:
            # No immediate profit. Must work the order.
            # Place limit sell at break_even + profit
            target_price = break_even + self.target_profit
            target_price = round(max(0.01, min(0.99, target_price)), 2)
            
            self.update_order("YES" if is_yes else "NO", market, outcome, target_price, self.positions["YES" if is_yes else "NO"])

    def update_order(self, side_key: str, market, outcome, price, size):
        """Place or update order."""
        # Simple implementation: Cancel replace if price differs
        # Production: Check active order price, modify if diff > threshold
        
        # Only demonstrating logic here
        logger.info(f"Managing {side_key}: Target Price {price}")
        
        if side_key not in self.orders:
            # Place new
            order = self.om._place_single_order_fixed(
                market, outcome, OrderSide.SELL, price, size
            )
            if order and order.status == OrderStatus.PLACED:
                self.orders[side_key] = order.order_id
        
    def check_fills(self, market):
        """Check if orders filled."""
        for side in ["YES", "NO"]:
            if side in self.orders:
                oid = self.orders[side]
                # In real bot, use self.om.check_order_status
                # Here we mock or would call API
                pass # Assume API check updates state
                
    def cancel_all(self):
        """Cleanup"""
        for oid in self.orders.values():
            self.om.cancel_order(oid)

    def cancel_order_for_side(self, side):
        if side in self.orders:
            self.om.cancel_order(self.orders[side])
            del self.orders[side]

if __name__ == "__main__":
    strategy = SplitMakerStrategy(budget_usdc=5.0)
    strategy.run()
