"""Main entry point for the Polymarket Limit Order Bot."""

import sys
import argparse
from logger import logger
from config import Config


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description="Polymarket Limit Order Bot - Automated liquidity provision for BTC 15m markets"
    )
    parser.add_argument(
        "--mode",
        choices=["bot", "dashboard", "both"],
        default="both",
        help="Run mode: bot only, dashboard only, or both (default: both)"
    )
    parser.add_argument(
        "--check-config",
        action="store_true",
        help="Check configuration and exit"
    )

    args = parser.parse_args()

    # Check configuration
    try:
        Config.validate()
        logger.info("Configuration validated successfully")

        if args.check_config:
            print("\n✓ Configuration is valid!")
            print(f"  - Wallet address will be derived from private key")
            print(f"  - Order size: ${Config.ORDER_SIZE_USD} per order")
            print(f"  - Spread offset: {Config.SPREAD_OFFSET}")
            print(f"  - Check interval: {Config.CHECK_INTERVAL_SECONDS}s")
            print(f"  - Dashboard: http://{Config.DASHBOARD_HOST}:{Config.DASHBOARD_PORT}")
            return 0

    except Exception as e:
        logger.error(f"Configuration error: {e}")
        print(f"\n✗ Configuration error: {e}")
        print("\nPlease check your .env file and ensure all required values are set.")
        return 1

    # Run based on mode
    try:
        if args.mode == "bot":
            logger.info("Starting bot-only mode")
            from bot import run_bot_loop
            run_bot_loop()

        elif args.mode == "dashboard":
            logger.info("Starting dashboard-only mode")
            from dashboard import run_dashboard
            run_dashboard()

        else:  # both
            logger.info("Starting bot with dashboard")
            from dashboard import run_dashboard
            run_dashboard()

    except KeyboardInterrupt:
        logger.info("\nShutdown requested by user")
        return 0
    except Exception as e:
        logger.error(f"Fatal error: {e}", exc_info=True)
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
