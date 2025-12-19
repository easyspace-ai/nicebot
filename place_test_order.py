"""Quick script to manually place test orders on a specific market."""

import sys
from order_manager import OrderManager
from market_discovery import MarketDiscovery
from config import Config
from logger import logger

def main():
    """Place test orders on the first available market."""

    print("\n" + "="*60)
    print("Manual Test Order Placement")
    print("="*60)
    print(f"Configuration:")
    print(f"  - Order price: $0.49")
    print(f"  - Order size: 10 shares")
    print(f"  - Total needed: $9.80 USDC (2 orders × 10 shares × $0.49)")
    print("="*60 + "\n")

    # Initialize
    logger.info("Initializing order manager...")
    order_manager = OrderManager(Config.PRIVATE_KEY)

    logger.info(f"Wallet address: {order_manager.address}")

    # Check balance
    balance = order_manager.get_usdc_balance()
    print(f"\nUSDC Balance: ${balance:.2f}")

    if balance == 0:
        print(f"\n[WARNING] Balance check failed (API credentials error)")
        print(f"  Will proceed with order placement anyway")
        print(f"  If wallet has insufficient USDC, the CLOB API will reject the order")
    elif balance < 9.80:
        print(f"\n[WARNING] Insufficient balance!")
        print(f"  Required: $9.80")
        print(f"  Current: ${balance:.2f}")
        print(f"  Missing: ${9.80 - balance:.2f}")
        print("\nPlease fund your wallet with at least $10 USDC on Polygon network.")
        return 1

    # Discover markets
    logger.info("\nDiscovering BTC 15m markets...")
    discovery = MarketDiscovery()
    markets = discovery.discover_btc_15m_markets()

    if not markets:
        print("\n[ERROR] No BTC 15m markets found!")
        return 1

    print(f"\nFound {len(markets)} markets")
    print("\nAvailable markets:")
    for i, market in enumerate(markets[:5], 1):
        print(f"  {i}. {market.market_slug}")
        print(f"     Starts: {market.start_datetime}")

    # Use first market
    market = markets[0]
    print(f"\nUsing market: {market.market_slug}")

    # Confirm
    response = input("\nPlace 2 test orders (1 Yes, 1 No) at $0.49, 10 shares each? (yes/no): ")
    if response.lower() not in ['yes', 'y']:
        print("Cancelled.")
        return 0

    # Place orders
    logger.info("\nPlacing test orders...")
    orders = order_manager.place_simple_test_orders(
        market=market,
        price=0.49,
        size=10.0
    )

    if orders:
        print(f"\n[SUCCESS] Placed {len(orders)} orders:")
        for order in orders:
            print(f"  - {order.side.value} {order.outcome} @ ${order.price:.2f}")
            print(f"    Size: {order.size:.2f} shares (${order.size_usd:.2f})")
            print(f"    Order ID: {order.order_id}")
            print(f"    Status: {order.status.value}")

        # Check new balance
        new_balance = order_manager.get_usdc_balance()
        print(f"\nNew USDC Balance: ${new_balance:.2f}")
        print(f"Used: ${balance - new_balance:.2f}")

        return 0
    else:
        print("\n[FAILED] Could not place orders. Check logs for details.")
        return 1


if __name__ == "__main__":
    sys.exit(main())
