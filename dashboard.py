"""FastAPI dashboard for monitoring the bot."""

import threading
import json
import time
from pathlib import Path
from collections import defaultdict
from datetime import datetime, timedelta
from fastapi import FastAPI, Request
from fastapi.responses import HTMLResponse, JSONResponse
from fastapi.templating import Jinja2Templates
from fastapi.staticfiles import StaticFiles
import uvicorn
from bot import get_bot_instance, run_bot_loop
from config import Config
from logger import logger
from models import OrderRecord, OrderStatus, OrderSide

app = FastAPI(title="Polymarket Limit Order Bot Dashboard")
templates = Jinja2Templates(directory="templates")

# Start bot in background thread
bot_thread = None

# Cache configuration
CACHE_TTL_SECONDS = 30
ORDER_HISTORY_FILE = "order_history.json"

_cache = {
    "order_history": None,
    "order_history_mtime": 0,
    "statistics": None,
    "statistics_computed_at": 0,
    "strategy_statistics": None,
    "strategy_statistics_computed_at": 0,
    "market_history": None,
    "market_history_computed_at": 0
}


def _get_cached_order_history() -> list[OrderRecord]:
    """Load order history with file-mtime caching."""
    try:
        file_path = Path(ORDER_HISTORY_FILE)
        if not file_path.exists():
            return []

        current_mtime = file_path.stat().st_mtime
        if (_cache["order_history"] is not None and
                _cache["order_history_mtime"] == current_mtime):
            return _cache["order_history"]

        with file_path.open("r") as f:
            order_history_data = json.load(f)
            for order in order_history_data:
                if order.get("strategy") is None:
                    order["strategy"] = "None"
            order_history = [OrderRecord(**order) for order in order_history_data]

        _cache["order_history"] = order_history
        _cache["order_history_mtime"] = current_mtime
        _cache["statistics"] = None
        _cache["strategy_statistics"] = None
        _cache["market_history"] = None
        return order_history
    except Exception as e:
        logger.error(f"Error loading order history: {e}", exc_info=True)
        return []


def start_bot_background():
    """Start bot in background thread."""
    global bot_thread
    if bot_thread is None or not bot_thread.is_alive():
        bot_thread = threading.Thread(target=run_bot_loop, daemon=True)
        bot_thread.start()
        logger.info("Bot started in background thread")


@app.on_event("startup")
async def startup_event():
    """Start bot when dashboard starts."""
    start_bot_background()


@app.get("/", response_class=HTMLResponse)
async def dashboard(request: Request):
    """Main dashboard page."""
    return templates.TemplateResponse("dashboard.html", {"request": request})


@app.get("/api/status")
async def get_status():
    """Get current bot status."""
    try:
        bot = get_bot_instance()
        logger.debug(f"API: bot instance id={id(bot)}, is_running={bot.state.is_running}")
        state = bot.get_state()
        now = datetime.now()

        # Check if bot has sufficient balance to place orders
        from config import Config
        from models import OrderStatus
        # Only need USDC for BUY orders (2 outcomes × 1 BUY side each)
        # SELL orders would require tokens we don't have yet
        min_balance_needed = Config.ORDER_SIZE_USD * 2
        has_sufficient_balance = state.usdc_balance >= min_balance_needed if state.usdc_balance is not None else False

        # Count failed orders with balance errors
        balance_error_count = 0
        for order in state.pending_orders:
            if (order.status == OrderStatus.FAILED and
                order.error_message and
                ('balance' in order.error_message.lower() or 'allowance' in order.error_message.lower())):
                balance_error_count += 1

        # Format data for JSON response
        return {
            "is_running": state.is_running,
            "last_check": (state.last_check or now).isoformat(),
            "next_check": (
                ((state.last_check or now) + timedelta(seconds=Config.CHECK_INTERVAL_SECONDS)).isoformat()
            ),
            "check_interval_seconds": Config.CHECK_INTERVAL_SECONDS,
            "usdc_balance": round(state.usdc_balance, 2),
            "total_pnl": round(state.total_pnl, 2),
            "error_count": state.error_count,
            "last_error": state.last_error,
            "active_markets_count": len(state.active_markets),
            "pending_orders_count": len(state.pending_orders),
            "wallet_address": bot.order_manager.address,
            "balance_warning": not has_sufficient_balance,  # NEW
            "balance_error_count": balance_error_count,  # NEW
            "min_balance_needed": min_balance_needed  # NEW
        }
    except Exception as e:
        logger.error(f"Error getting status: {e}")
        return {"error": str(e)}


@app.get("/api/markets")
async def get_markets():
    """Get active markets."""
    try:
        bot = get_bot_instance()
        state = bot.get_state()

        markets_data = []
        for market in state.active_markets:
            time_until_start = market.time_until_start

            markets_data.append({
                "market_slug": market.market_slug,
                "question": market.question,
                "start_timestamp": market.start_timestamp,
                "start_datetime": market.start_datetime.isoformat(),
                "end_datetime": market.end_datetime.isoformat(),
                "time_until_start": round(time_until_start),
                "time_until_start_formatted": format_time_delta(time_until_start),
                "is_active": market.is_active,
                "is_resolved": market.is_resolved,
                "outcomes": [
                    {
                        "outcome": o.outcome,
                        "price": round(o.price, 3) if o.price else None,
                        "best_bid": round(o.best_bid, 3) if o.best_bid else None,
                        "best_ask": round(o.best_ask, 3) if o.best_ask else None
                    }
                    for o in market.outcomes
                ],
                "orders_placed": bot.orders_placed.get(market.condition_id, False)
            })

        # Sort by start time and limit to 10 nearest markets
        markets_data.sort(key=lambda m: m["start_timestamp"])
        markets_data = markets_data[:10]

        return {"markets": markets_data}

    except Exception as e:
        logger.error(f"Error getting markets: {e}")
        return {"error": str(e), "markets": []}


@app.get("/api/orders")
async def get_orders():
    """Get order information."""
    try:
        bot = get_bot_instance()
        state = bot.get_state()

        pending_orders = [
            {
                "order_id": o.order_id[:16] + "...",
                "market_slug": o.market_slug,
                "outcome": o.outcome,
                "side": o.side.value,
                "price": round(o.price, 3),
                "size": round(o.size, 2),
                "size_usd": round(o.size_usd, 2),
                "status": o.status.value,
                "strategy": o.strategy,
                "created_at": o.created_at.isoformat(),
                "filled_at": o.filled_at.isoformat() if o.filled_at else None
            }
            for o in state.pending_orders
        ]

        recent_orders = [
            {
                "order_id": o.order_id[:16] + "...",
                "market_slug": o.market_slug,
                "outcome": o.outcome,
                "side": o.side.value,
                "price": round(o.price, 3),
                "size": round(o.size, 2),
                "size_usd": round(o.size_usd, 2),
                "status": o.status.value,
                "strategy": o.strategy,
                "created_at": o.created_at.isoformat(),
                "filled_at": o.filled_at.isoformat() if o.filled_at else None,
                "error_message": o.error_message
            }
            for o in state.recent_orders[:100]  # Last 100 orders
        ]

        return {
            "pending_orders": pending_orders,
            "recent_orders": recent_orders
        }

    except Exception as e:
        logger.error(f"Error getting orders: {e}")
        return {"error": str(e), "pending_orders": [], "recent_orders": []}


@app.get("/api/market-history")
async def get_market_history():
    """Get market-level order history (one row per market)."""
    try:
        now = time.time()
        if (_cache["market_history"] is not None and
                now - _cache["market_history_computed_at"] < CACHE_TTL_SECONDS):
            return _cache["market_history"]

        order_history = _get_cached_order_history()

        if not order_history:
            result = {"markets": []}
            _cache["market_history"] = result
            _cache["market_history_computed_at"] = now
            return result

        markets = defaultdict(list)
        for order in order_history:
            markets[order.condition_id].append(order)

        results = []
        for condition_id, orders in markets.items():
            market_slug = orders[0].market_slug if orders else condition_id[:16]
            strategy = orders[0].strategy or "None"

            total_cost = 0.0
            total_revenue = 0.0

            has_merge = False
            has_open = False
            has_partial = False
            has_filled = False
            has_cancelled = False
            has_failed = False

            created_at_values = []
            primary_orders = [o for o in orders if o.transaction_type == "BUY"]
            if not primary_orders:
                primary_orders = orders
            total_count = len(primary_orders)
            filled_count = 0

            for order in orders:
                created_at_values.append(order.created_at)
                if order.transaction_type == "MERGE" and order.status == OrderStatus.FILLED:
                    has_merge = True

                if order.status == OrderStatus.PARTIALLY_FILLED:
                    has_partial = True
                if order.status == OrderStatus.FILLED:
                    has_filled = True
                if order.status in [OrderStatus.PLACED, OrderStatus.PARTIALLY_FILLED]:
                    has_open = True
                if order.status == OrderStatus.CANCELLED:
                    has_cancelled = True
                if order.status == OrderStatus.FAILED:
                    has_failed = True

                if order.status in [OrderStatus.FILLED, OrderStatus.PARTIALLY_FILLED]:
                    if order.side == OrderSide.BUY:
                        if order.cost_usd is not None:
                            total_cost += float(order.cost_usd)
                        elif order.size_matched is not None:
                            total_cost += float(order.price) * float(order.size_matched)
                        else:
                            total_cost += float(order.size_usd)
                    elif order.side == OrderSide.SELL:
                        if order.revenue_usd is not None:
                            total_revenue += float(order.revenue_usd)
                        elif order.size_matched is not None:
                            total_revenue += float(order.price) * float(order.size_matched)
                        else:
                            total_revenue += float(order.size_usd)

            for order in primary_orders:
                if order.status in [OrderStatus.FILLED, OrderStatus.PARTIALLY_FILLED]:
                    filled_count += 1

            # Status summary: show filled count for primary orders
            if total_count > 0:
                status_summary = f"FILLED {filled_count}/{total_count}"
            elif has_filled:
                status_summary = "FILLED"
            elif has_cancelled:
                status_summary = "CANCELLED"
            elif has_failed:
                status_summary = "FAILED"
            elif has_open:
                status_summary = "OPEN"
            else:
                status_summary = "UNKNOWN"

            # Result summary
            if has_open:
                result_summary = "OPEN"
            elif has_merge:
                result_summary = "SUCCESS"
            elif total_revenue >= Config.ORDER_SIZE_USD:
                result_summary = "SUCCESS"
            elif total_revenue > 0:
                result_summary = "FAILED"
            elif total_cost > 0 and total_revenue == 0:
                result_summary = "FAILED"
            else:
                result_summary = "N/A"

            # For OPEN markets, suppress cost/PNL until fills/exit finalize
            total_pnl = total_revenue - total_cost
            if has_open:
                total_cost = 0.0
                total_revenue = 0.0
                total_pnl = 0.0

            created_at = min(created_at_values) if created_at_values else None

            results.append({
                "market_slug": market_slug,
                "condition_id": condition_id,
                "strategy": strategy,
                "status": status_summary,
                "result": result_summary,
                "total_cost": round(total_cost, 2),
                "total_revenue": round(total_revenue, 2),
                "pnl": round(total_pnl, 2),
                "filled_count": filled_count,
                "total_count": total_count,
                "created_at": created_at.isoformat() if created_at else None
            })

        # Sort by created_at desc, then limit to 100
        results.sort(key=lambda r: r.get("created_at") or "", reverse=True)
        result = {"markets": results[:100]}
        _cache["market_history"] = result
        _cache["market_history_computed_at"] = now
        return result

    except Exception as e:
        logger.error(f"Error getting market history: {e}", exc_info=True)
        return {"markets": []}

@app.get("/api/statistics")
async def get_statistics():
    """Get trading statistics."""
    try:
        now = time.time()
        if (_cache["statistics"] is not None and
                now - _cache["statistics_computed_at"] < CACHE_TTL_SECONDS):
            return _cache["statistics"]

        bot = get_bot_instance()
        if not bot:
            result = {
                "total_markets": 0,
                "successful_trades": 0,
                "unsuccessful_trades": 0,
                "total_pnl": 0.0
            }
            _cache["statistics"] = result
            _cache["statistics_computed_at"] = now
            return result

        order_history = _get_cached_order_history()

        # Group orders by condition_id (market)
        markets = defaultdict(list)

        for order in order_history:
            markets[order.condition_id].append(order)

        # Analyze each market
        total_markets = len(markets)
        successful_trades = 0
        unsuccessful_trades = 0
        total_pnl = 0.0

        for condition_id, orders in markets.items():
            # Calculate filled amounts per outcome
            yes_filled = 0.0
            no_filled = 0.0

            for order in orders:
                if order.status in [OrderStatus.FILLED, OrderStatus.PARTIALLY_FILLED]:
                    outcome_upper = order.outcome.strip().upper()
                    if outcome_upper in ["YES", "UP"]:
                        yes_filled += order.size
                    elif outcome_upper in ["NO", "DOWN"]:
                        no_filled += order.size

                if order.pnl_usd is not None:
                    total_pnl += float(order.pnl_usd)

            # Classify the market
            if yes_filled > 0 and no_filled > 0:
                # Both orders filled = successful
                successful_trades += 1
            else:
                # No orders filled OR only one order filled = unsuccessful
                unsuccessful_trades += 1

        result = {
            "total_markets": total_markets,
            "successful_trades": successful_trades,
            "unsuccessful_trades": unsuccessful_trades,
            "total_pnl": round(total_pnl, 2)
        }
        _cache["statistics"] = result
        _cache["statistics_computed_at"] = now
        return result

    except Exception as e:
        logger.error(f"Error getting statistics: {e}", exc_info=True)
        return {
            "total_markets": 0,
            "successful_trades": 0,
            "unsuccessful_trades": 0,
            "total_pnl": 0.0
        }


@app.get("/api/strategy-statistics")
async def get_strategy_statistics():
    """Get trading statistics grouped by strategy."""
    try:
        now = time.time()
        if (_cache["strategy_statistics"] is not None and
                now - _cache["strategy_statistics_computed_at"] < CACHE_TTL_SECONDS):
            return _cache["strategy_statistics"]

        bot = get_bot_instance()
        if not bot:
            result = {"strategies": []}
            _cache["strategy_statistics"] = result
            _cache["strategy_statistics_computed_at"] = now
            return result

        order_history = _get_cached_order_history()

        if not order_history:
            result = {"strategies": []}
            _cache["strategy_statistics"] = result
            _cache["strategy_statistics_computed_at"] = now
            return result

        strategies = defaultdict(list)
        for order in order_history:
            strategy_name = order.strategy or "None"
            strategies[strategy_name].append(order)

        results = []
        for strategy_name, orders in strategies.items():
            markets = defaultdict(list)
            for order in orders:
                markets[order.condition_id].append(order)

            total_markets = len(markets)
            successful_trades = 0
            unsuccessful_trades = 0
            total_pnl = 0.0

            for condition_id, market_orders in markets.items():
                yes_filled = 0.0
                no_filled = 0.0

                for order in market_orders:
                    if order.pnl_usd is not None:
                        total_pnl += order.pnl_usd

                    if order.status in [OrderStatus.FILLED, OrderStatus.PARTIALLY_FILLED]:
                        outcome_upper = order.outcome.strip().upper()
                        if outcome_upper in ["YES", "UP"]:
                            yes_filled += order.size
                        elif outcome_upper in ["NO", "DOWN"]:
                            no_filled += order.size

                if yes_filled > 0 and no_filled > 0:
                    successful_trades += 1
                else:
                    unsuccessful_trades += 1

            results.append({
                "strategy_name": strategy_name,
                "total_markets": total_markets,
                "successful_trades": successful_trades,
                "unsuccessful_trades": unsuccessful_trades,
                "total_pnl": total_pnl
            })

        results.sort(key=lambda r: r["strategy_name"])
        result = {"strategies": results}
        _cache["strategy_statistics"] = result
        _cache["strategy_statistics_computed_at"] = now
        return result

    except Exception as e:
        logger.error(f"Error getting strategy statistics: {e}", exc_info=True)
        return {"strategies": []}


@app.get("/api/logs")
async def get_logs():
    """Get recent log entries."""
    try:
        file_path = Path(Config.LOG_FILE)
        if not file_path.exists():
            return {"logs": []}

        tail_bytes = 10000
        file_size = file_path.stat().st_size
        with file_path.open("rb") as f:
            if file_size > tail_bytes:
                f.seek(-tail_bytes, 2)
            tail_content = f.read().decode("utf-8", errors="ignore")
        lines = tail_content.splitlines()
        recent_lines = lines[-50:] if len(lines) > 50 else lines
        return {"logs": recent_lines}

    except Exception as e:
        return {"error": str(e), "logs": []}


def format_time_delta(seconds: float) -> str:
    """Format time delta in human-readable format."""
    if seconds < 0:
        return "Started"

    if seconds < 60:
        return f"{int(seconds)}s"
    elif seconds < 3600:
        minutes = int(seconds / 60)
        secs = int(seconds % 60)
        return f"{minutes}m {secs}s"
    else:
        hours = int(seconds / 3600)
        minutes = int((seconds % 3600) / 60)
        return f"{hours}h {minutes}m"


def run_dashboard():
    """Run the dashboard server."""
    logger.info(f"Starting dashboard on {Config.DASHBOARD_HOST}:{Config.DASHBOARD_PORT}")
    uvicorn.run(
        app,
        host=Config.DASHBOARD_HOST,
        port=Config.DASHBOARD_PORT,
        log_level="info"
    )


if __name__ == "__main__":
    run_dashboard()
