"""Configuration management for Polymarket Limit Order Bot."""

import os
from typing import Optional
from dotenv import load_dotenv

load_dotenv()


class Config:
    """Bot configuration loaded from environment variables."""

    # Polymarket Configuration
    PRIVATE_KEY: str = os.getenv("PRIVATE_KEY", "")
    CHAIN_ID: int = int(os.getenv("CHAIN_ID", "137"))
    SIGNATURE_TYPE: str = os.getenv("SIGNATURE_TYPE", "EOA")
    FUNDER_ADDRESS: Optional[str] = os.getenv("FUNDER_ADDRESS")

    # Bot Configuration
    ORDER_SIZE_USD: float = float(os.getenv("ORDER_SIZE_USD", "10.0"))
    SPREAD_OFFSET: float = float(os.getenv("SPREAD_OFFSET", "0.01"))
    CHECK_INTERVAL_SECONDS: int = int(os.getenv("CHECK_INTERVAL_SECONDS", "60"))
    ORDER_PLACEMENT_MIN_MINUTES: int = int(os.getenv("ORDER_PLACEMENT_MIN_MINUTES", "10"))
    ORDER_PLACEMENT_MAX_MINUTES: int = int(os.getenv("ORDER_PLACEMENT_MAX_MINUTES", "20"))
    REDEEM_CHECK_INTERVAL_SECONDS: int = int(os.getenv("REDEEM_CHECK_INTERVAL_SECONDS", "60"))
    MIN_SELL_PRICE: float = float(os.getenv("MIN_SELL_PRICE", "0.10"))
    MARKET_SELL_DISCOUNT: float = float(os.getenv("MARKET_SELL_DISCOUNT", "0.02"))

    # Strategy Configuration
    STRATEGY_NAME: str = os.getenv("STRATEGY_NAME", "quick_exit_7_5min")
    ORDER_MODE: str = os.getenv("ORDER_MODE", "test")  # test | liquidity

    # Strategy Parameters
    STRATEGY_PARAMS = {
        "quick_exit_7_5min": {
            "exit_timeout_seconds": 450,  # 7.5 minutes after market start
            "cancel_unfilled": True,
            "market_sell_filled": True,
            "enabled": True
        }
    }

    @classmethod
    def get_strategy_config(cls, strategy_name: str = None) -> dict:
        """Get configuration for a specific strategy."""
        name = strategy_name or cls.STRATEGY_NAME
        return cls.STRATEGY_PARAMS.get(name, {})

    # API Configuration
    GAMMA_API_BASE_URL: str = os.getenv("GAMMA_API_BASE_URL", "https://gamma-api.polymarket.com")
    CLOB_API_URL: str = os.getenv("CLOB_API_URL", "https://clob.polymarket.com")
    RPC_URL: str = os.getenv("RPC_URL", "https://polygon-rpc.com")
    POLYMARKET_API_KEY: Optional[str] = os.getenv("POLYMARKET_API_KEY")
    POLYMARKET_API_SECRET: Optional[str] = os.getenv("POLYMARKET_API_SECRET")
    POLYMARKET_API_PASSPHRASE: Optional[str] = os.getenv("POLYMARKET_API_PASSPHRASE", "")

    # Dashboard Configuration
    DASHBOARD_HOST: str = os.getenv("DASHBOARD_HOST", "0.0.0.0")
    DASHBOARD_PORT: int = int(os.getenv("DASHBOARD_PORT", "8000"))

    # Logging
    LOG_LEVEL: str = os.getenv("LOG_LEVEL", "INFO")
    LOG_FILE: str = os.getenv("LOG_FILE", "bot.log")

    @classmethod
    def validate(cls) -> bool:
        """Validate required configuration."""
        if not cls.PRIVATE_KEY:
            raise ValueError("PRIVATE_KEY is required in .env file")
        if cls.ORDER_SIZE_USD <= 0:
            raise ValueError("ORDER_SIZE_USD must be positive")
        if cls.SPREAD_OFFSET <= 0:
            raise ValueError("SPREAD_OFFSET must be positive")
        return True


# Validate configuration on import
Config.validate()
