package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"limitorderbot/internal/config"
)

func newCheckConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check-config",
		Short: "检查 .env 配置并退出",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			fmt.Println("\n✓ Configuration is valid!")
			fmt.Printf("  - Wallet address will be derived from private key\n")
			fmt.Printf("  - Order size: $%.2f per order\n", cfg.OrderSizeUSD)
			fmt.Printf("  - Spread offset: %.4f\n", cfg.SpreadOffset)
			fmt.Printf("  - Check interval: %ds\n", cfg.CheckIntervalSeconds)
			fmt.Printf("  - Dashboard: http://%s:%d\n", cfg.DashboardHost, cfg.DashboardPort)
			return nil
		},
	}
}
