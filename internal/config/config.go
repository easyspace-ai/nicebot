package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/joho/godotenv"
)

type StrategyConfig struct {
	ExitTimeoutSeconds int  `json:"exit_timeout_seconds"`
	CancelUnfilled     bool `json:"cancel_unfilled"`
	MarketSellFilled   bool `json:"market_sell_filled"`
	Enabled            bool `json:"enabled"`
}

type Config struct {
	// Polymarket
	PrivateKey    string
	ChainID       int64
	SignatureType string
	FunderAddress string

	// Bot
	OrderSizeUSD               float64
	SpreadOffset               float64
	CheckIntervalSeconds       int
	OrderPlacementMinMinutes   int
	OrderPlacementMaxMinutes   int
	RedeemCheckIntervalSeconds int
	MinSellPrice               float64
	MarketSellDiscount         float64
	StrategyName               string
	OrderMode                  string
	GammaAPIBaseURL            string
	ClobAPIURL                 string
	RPCURL                     string
	PolymarketAPIKey           string
	PolymarketAPISecret        string
	PolymarketAPIPassphrase    string
	DashboardHost              string
	DashboardPort              int
	LogLevel                   string
	LogFile                    string
	Strategies                 map[string]StrategyConfig
}

var (
	loadedCfg Config
	loadOnce  sync.Once
	loadErr   error
)

func MustLoad() Config {
	cfg, err := Load()
	if err != nil {
		panic(err)
	}
	return cfg
}

func Load() (Config, error) {
	loadOnce.Do(func() {
		// Best-effort .env loading to match python behavior.
		_ = godotenv.Load()

		loadedCfg = Config{
			PrivateKey:    os.Getenv("PRIVATE_KEY"),
			ChainID:       mustInt64("CHAIN_ID", 137),
			SignatureType: envOr("SIGNATURE_TYPE", "EOA"),
			FunderAddress: os.Getenv("FUNDER_ADDRESS"),

			OrderSizeUSD:               mustFloat("ORDER_SIZE_USD", 10.0),
			SpreadOffset:               mustFloat("SPREAD_OFFSET", 0.01),
			CheckIntervalSeconds:       mustInt("CHECK_INTERVAL_SECONDS", 60),
			OrderPlacementMinMinutes:   mustInt("ORDER_PLACEMENT_MIN_MINUTES", 10),
			OrderPlacementMaxMinutes:   mustInt("ORDER_PLACEMENT_MAX_MINUTES", 20),
			RedeemCheckIntervalSeconds: mustInt("REDEEM_CHECK_INTERVAL_SECONDS", 60),
			MinSellPrice:               mustFloat("MIN_SELL_PRICE", 0.10),
			MarketSellDiscount:         mustFloat("MARKET_SELL_DISCOUNT", 0.02),

			StrategyName: envOr("STRATEGY_NAME", "quick_exit_7_5min"),
			OrderMode:    envOr("ORDER_MODE", "test"),

			GammaAPIBaseURL:         envOr("GAMMA_API_BASE_URL", "https://gamma-api.polymarket.com"),
			ClobAPIURL:              envOr("CLOB_API_URL", "https://clob.polymarket.com"),
			RPCURL:                  envOr("RPC_URL", "https://polygon-rpc.com"),
			PolymarketAPIKey:        os.Getenv("POLYMARKET_API_KEY"),
			PolymarketAPISecret:     os.Getenv("POLYMARKET_API_SECRET"),
			PolymarketAPIPassphrase: envOr("POLYMARKET_API_PASSPHRASE", ""),

			DashboardHost: envOr("DASHBOARD_HOST", "0.0.0.0"),
			DashboardPort: mustInt("DASHBOARD_PORT", 8000),

			LogLevel: envOr("LOG_LEVEL", "INFO"),
			LogFile:  envOr("LOG_FILE", "bot.log"),

			Strategies: map[string]StrategyConfig{
				"quick_exit_7_5min": {
					ExitTimeoutSeconds: 450,
					CancelUnfilled:     true,
					MarketSellFilled:   true,
					Enabled:            true,
				},
			},
		}

		loadErr = validate(loadedCfg)
	})

	return loadedCfg, loadErr
}

func (c Config) Strategy() (StrategyConfig, bool) {
	s, ok := c.Strategies[c.StrategyName]
	return s, ok
}

func validate(c Config) error {
	if c.PrivateKey == "" {
		return errors.New("PRIVATE_KEY is required in .env file")
	}
	if c.OrderSizeUSD <= 0 {
		return errors.New("ORDER_SIZE_USD must be positive")
	}
	if c.SpreadOffset <= 0 {
		return errors.New("SPREAD_OFFSET must be positive")
	}
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustInt(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

func mustInt64(key string, def int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return def
	}
	return v
}

func mustFloat(key string, def float64) float64 {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return def
	}
	return v
}

func (c Config) String() string {
	return fmt.Sprintf("chain=%d signature=%s orderSize=%.2f spread=%.4f", c.ChainID, c.SignatureType, c.OrderSizeUSD, c.SpreadOffset)
}
