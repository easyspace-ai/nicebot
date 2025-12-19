"""Test script to verify Polymarket connection and configuration."""

import sys
from config import Config
from logger import logger


def test_configuration():
    """Test configuration loading."""
    print("\n" + "="*60)
    print("CONFIGURATION TEST")
    print("="*60)

    try:
        Config.validate()
        print("[OK] Configuration loaded successfully")
        print(f"  - Chain ID: {Config.CHAIN_ID}")
        print(f"  - Signature Type: {Config.SIGNATURE_TYPE}")
        print(f"  - Order Size: ${Config.ORDER_SIZE_USD}")
        print(f"  - Spread Offset: {Config.SPREAD_OFFSET}")
        print(f"  - Check Interval: {Config.CHECK_INTERVAL_SECONDS}s")
        return True
    except Exception as e:
        print(f"[FAIL] Configuration error: {e}")
        return False


def test_gamma_api():
    """Test connection to Gamma API."""
    print("\n" + "="*60)
    print("GAMMA API TEST")
    print("="*60)

    try:
        from market_discovery import MarketDiscovery

        discovery = MarketDiscovery()
        print("[OK] Market discovery client initialized")

        print("  Fetching markets...")
        markets = discovery.discover_btc_15m_markets()

        print(f"[OK] Successfully connected to Gamma API")
        print(f"  - Found {len(markets)} BTC 15m markets")

        if markets:
            print("\n  Recent markets:")
            for market in markets[:3]:
                print(f"    - {market.market_slug}")
                print(f"      Start: {market.start_datetime}")

        return True

    except Exception as e:
        print(f"[FAIL] Gamma API error: {e}")
        logger.error("Gamma API test failed", exc_info=True)
        return False


def test_clob_client():
    """Test CLOB client initialization."""
    print("\n" + "="*60)
    print("CLOB CLIENT TEST")
    print("="*60)

    try:
        from order_manager import OrderManager

        print("  Initializing CLOB client...")
        manager = OrderManager(Config.PRIVATE_KEY)

        print(f"[OK] CLOB client initialized")
        print(f"  - Wallet address: {manager.address}")

        print("  Fetching USDC balance...")
        balance = manager.get_usdc_balance()

        print(f"[OK] Successfully connected to CLOB API")
        print(f"  - USDC Balance: ${balance:.2f}")

        if balance < Config.ORDER_SIZE_USD * 4:
            print(f"\n  [WARNING] Low balance!")
            print(f"    You need at least ${Config.ORDER_SIZE_USD * 4:.2f} USDC")
            print(f"    to place all orders (4 orders × ${Config.ORDER_SIZE_USD})")

        return True

    except Exception as e:
        print(f"[FAIL] CLOB client error: {e}")
        logger.error("CLOB client test failed", exc_info=True)
        return False


def main():
    """Run all tests."""
    print("\n" + "="*60)
    print("   Polymarket Limit Order Bot - Connection Test")
    print("="*60)

    results = []

    # Test configuration
    results.append(("Configuration", test_configuration()))

    # Test Gamma API
    results.append(("Gamma API", test_gamma_api()))

    # Test CLOB client
    results.append(("CLOB Client", test_clob_client()))

    # Summary
    print("\n" + "="*60)
    print("TEST SUMMARY")
    print("="*60)

    all_passed = True
    for name, passed in results:
        status = "[PASSED]" if passed else "[FAILED]"
        print(f"{name:20s} {status}")
        if not passed:
            all_passed = False

    print("="*60)

    if all_passed:
        print("\n*** All tests passed! Your bot is ready to run.")
        print("\nTo start the bot:")
        print("  py main.py")
        print("\nOr use the run script:")
        print("  Windows: run.bat")
        print("  Unix/Mac: ./run.sh")
        return 0
    else:
        print("\n*** Some tests failed. Please fix the errors above.")
        print("\nCommon issues:")
        print("  - Check your PRIVATE_KEY in .env")
        print("  - Ensure you have USDC in your wallet")
        print("  - Verify your internet connection")
        return 1


if __name__ == "__main__":
    sys.exit(main())
