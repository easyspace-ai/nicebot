"""Data models for the Polymarket bot."""

from datetime import datetime
from typing import Optional, List
from pydantic import BaseModel, Field
from enum import Enum


class OrderSide(str, Enum):
    """Order side enumeration."""
    BUY = "BUY"
    SELL = "SELL"


class OrderStatus(str, Enum):
    """Order status enumeration."""
    PENDING = "PENDING"
    PLACED = "PLACED"
    FILLED = "FILLED"
    PARTIALLY_FILLED = "PARTIALLY_FILLED"
    CANCELLED = "CANCELLED"
    FAILED = "FAILED"


class Outcome(BaseModel):
    """Market outcome data."""
    token_id: str
    outcome: str  # "Yes" or "No"
    price: Optional[float] = None
    best_bid: Optional[float] = None
    best_ask: Optional[float] = None


class Market(BaseModel):
    """BTC 15-minute market data."""
    condition_id: str
    market_slug: str
    question: str
    start_timestamp: int
    end_timestamp: int
    outcomes: List[Outcome] = []
    is_active: bool = False
    is_resolved: bool = False

    @property
    def start_datetime(self) -> datetime:
        """Get market start time as datetime."""
        return datetime.fromtimestamp(self.start_timestamp)

    @property
    def end_datetime(self) -> datetime:
        """Get market end time as datetime."""
        return datetime.fromtimestamp(self.end_timestamp)

    @property
    def time_until_start(self) -> float:
        """Get seconds until market starts."""
        return self.start_timestamp - datetime.now().timestamp()

    @property
    def should_place_orders(self) -> bool:
        """Check if it's time to place orders (5 minutes before start)."""
        from config import Config
        seconds_until_start = self.time_until_start
        target_seconds = Config.ORDER_PLACEMENT_MINUTES_BEFORE * 60
        # Place orders between 5-6 minutes before to avoid timing issues
        return target_seconds <= seconds_until_start <= (target_seconds + 60)


class OrderRecord(BaseModel):
    """Record of a placed order."""
    order_id: str
    market_slug: str
    condition_id: str
    token_id: str
    outcome: str
    side: OrderSide
    price: float
    size: float
    size_usd: float
    status: OrderStatus = OrderStatus.PENDING
    created_at: datetime = Field(default_factory=datetime.now)
    filled_at: Optional[datetime] = None
    error_message: Optional[str] = None


class BotState(BaseModel):
    """Current bot state for dashboard."""
    is_running: bool = False
    last_check: Optional[datetime] = None
    active_markets: List[Market] = []
    pending_orders: List[OrderRecord] = []
    recent_orders: List[OrderRecord] = []
    usdc_balance: float = 0.0
    total_pnl: float = 0.0
    error_count: int = 0
    last_error: Optional[str] = None
