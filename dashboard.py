"""FastAPI dashboard for monitoring the bot."""

import threading
from datetime import datetime
from fastapi import FastAPI, Request
from fastapi.responses import HTMLResponse, JSONResponse
from fastapi.templating import Jinja2Templates
from fastapi.staticfiles import StaticFiles
import uvicorn
from bot import get_bot_instance, run_bot_loop
from config import Config
from logger import logger

app = FastAPI(title="Polymarket Limit Order Bot Dashboard")
templates = Jinja2Templates(directory="templates")

# Start bot in background thread
bot_thread = None


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
            "last_check": state.last_check.isoformat() if state.last_check else None,
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


@app.get("/api/logs")
async def get_logs():
    """Get recent log entries."""
    try:
        # Read last 50 lines from log file
        with open(Config.LOG_FILE, "r") as f:
            lines = f.readlines()
            recent_lines = lines[-50:] if len(lines) > 50 else lines

        return {"logs": [line.strip() for line in recent_lines]}

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
