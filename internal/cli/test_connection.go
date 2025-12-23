package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"limitorderbot/internal/chain"
	"limitorderbot/internal/clob"
	"limitorderbot/internal/config"
	"limitorderbot/internal/gamma"
)

func newTestConnectionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test-connection",
		Short: "测试 Gamma/CLOB/RPC 连接",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			fmt.Println("\n" + repeat("=", 60))
			fmt.Println("CONFIGURATION TEST")
			fmt.Println(repeat("=", 60))
			fmt.Println("[OK] Configuration loaded successfully")
			fmt.Printf("  - Chain ID: %d\n", cfg.ChainID)
			fmt.Printf("  - Signature Type: %s\n", cfg.SignatureType)
			fmt.Printf("  - Order Size: $%.2f\n", cfg.OrderSizeUSD)
			fmt.Printf("  - Spread Offset: %.4f\n", cfg.SpreadOffset)
			fmt.Printf("  - Check Interval: %ds\n", cfg.CheckIntervalSeconds)

			fmt.Println("\n" + repeat("=", 60))
			fmt.Println("GAMMA API TEST")
			fmt.Println(repeat("=", 60))
			disc := gamma.New(cfg.GammaAPIBaseURL)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			markets, err := disc.DiscoverBTC15mMarkets(ctx)
			if err != nil {
				return fmt.Errorf("[FAIL] Gamma API error: %w", err)
			}
			fmt.Println("[OK] Successfully connected to Gamma API")
			fmt.Printf("  - Found %d BTC 15m markets\n", len(markets))
			for i := 0; i < len(markets) && i < 3; i++ {
				fmt.Printf("    - %s\n", markets[i].MarketSlug)
				fmt.Printf("      Start: %s\n", markets[i].StartTime().Format(time.RFC3339))
			}

			fmt.Println("\n" + repeat("=", 60))
			fmt.Println("CLOB CLIENT TEST")
			fmt.Println(repeat("=", 60))
			cc, err := clob.NewClient(cfg.ClobAPIURL, cfg.ChainID, cfg.PrivateKey, cfg.SignatureType, cfg.FunderAddress)
			if err != nil {
				return fmt.Errorf("[FAIL] CLOB client init error: %w", err)
			}
			fmt.Printf("[OK] CLOB signer initialized\n")
			fmt.Printf("  - Wallet address: %s\n", cc.Address())

			// Derive creds (best-effort; some users run read-only)
			ctx2, cancel2 := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel2()
			creds, err := cc.CreateOrDeriveAPICreds(ctx2, 0)
			if err == nil && creds.APIKey != "" {
				cc.SetCreds(creds)
				fmt.Println("[OK] CLOB API creds derived")
			} else {
				fmt.Printf("[WARNING] Could not derive CLOB API creds (read-only OK): %v\n", err)
			}

			ch, err := chain.New(cfg.RPCURL, cfg.PrivateKey, cfg.ChainID)
			if err != nil {
				return fmt.Errorf("[FAIL] RPC client init error: %w", err)
			}
			defer ch.Close()
			ctx3, cancel3 := chain.WithTimeout(context.Background(), 20*time.Second)
			defer cancel3()
			bal, err := ch.USDCBalance(ctx3)
			if err != nil {
				return fmt.Errorf("[FAIL] USDC balance error: %w", err)
			}
			fmt.Println("[OK] Successfully connected to RPC")
			fmt.Printf("  - USDC Balance: $%.2f\n", bal)

			return nil
		},
	}
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
