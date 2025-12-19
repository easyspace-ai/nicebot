"""Track markets and their condition IDs for future redemption."""
import json
import os
from dataclasses import dataclass, asdict
from typing import List, Optional
from datetime import datetime

@dataclass
class TrackedMarket:
    """Market tracking data for redemption."""
    market_slug: str
    condition_id: str
    question: str
    end_time: int
    orders_placed: bool = False
    resolved: bool = False
    resolution_time: Optional[int] = None
    winning_outcome: Optional[str] = None

class MarketTracker:
    """Track markets for order placement and redemption."""

    def __init__(self, data_file: str = "tracked_markets.json"):
        self.data_file = data_file
        self.markets: List[TrackedMarket] = []
        self.load()

    def load(self):
        """Load tracked markets from file."""
        if os.path.exists(self.data_file):
            try:
                with open(self.data_file, 'r') as f:
                    data = json.load(f)
                    self.markets = [TrackedMarket(**m) for m in data]
            except Exception as e:
                print(f"Error loading tracked markets: {e}")
                self.markets = []
        else:
            self.markets = []

    def save(self):
        """Save tracked markets to file."""
        try:
            with open(self.data_file, 'w') as f:
                data = [asdict(m) for m in self.markets]
                json.dump(data, f, indent=2)
        except Exception as e:
            print(f"Error saving tracked markets: {e}")

    def add_market(self, market_slug: str, condition_id: str, question: str, end_time: int):
        """Add a market to tracking."""
        # Check if already tracked
        for m in self.markets:
            if m.market_slug == market_slug:
                return

        market = TrackedMarket(
            market_slug=market_slug,
            condition_id=condition_id,
            question=question,
            end_time=end_time
        )
        self.markets.append(market)
        self.save()

    def mark_orders_placed(self, market_slug: str):
        """Mark that orders were placed for this market."""
        for m in self.markets:
            if m.market_slug == market_slug:
                m.orders_placed = True
                self.save()
                break

    def mark_resolved(self, market_slug: str, winning_outcome: Optional[str] = None):
        """Mark a market as resolved."""
        for m in self.markets:
            if m.market_slug == market_slug:
                m.resolved = True
                m.resolution_time = int(datetime.now().timestamp())
                m.winning_outcome = winning_outcome
                self.save()
                break

    def get_unredeemed_markets(self) -> List[TrackedMarket]:
        """Get markets that are resolved but not yet redeemed."""
        return [m for m in self.markets if m.resolved and m.orders_placed]

    def get_market(self, market_slug: str) -> Optional[TrackedMarket]:
        """Get a specific market."""
        for m in self.markets:
            if m.market_slug == market_slug:
                return m
        return None

    def remove_market(self, market_slug: str):
        """Remove a market from tracking (after redemption)."""
        self.markets = [m for m in self.markets if m.market_slug != market_slug]
        self.save()
