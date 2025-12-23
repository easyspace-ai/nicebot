package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"limitorderbot/internal/chain"
	"limitorderbot/internal/config"
)

func newUSDCCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usdc",
		Short: "USDC / USDC.e 排障工具（等价 check_all_usdc.py）",
	}
	cmd.AddCommand(newUSDCCheckCmd())
	return cmd
}

func newUSDCCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "对比 USDC.e 与 Polygon 原生 USDC 余额",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			ch, err := chain.New(cfg.RPCURL, cfg.PrivateKey, cfg.ChainID)
			if err != nil {
				return err
			}
			defer ch.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			usdcE := common.HexToAddress(chain.USDCeAddress)
			usdc := common.HexToAddress(chain.USDCAddress)

			bE, err := ch.ERC20BalanceFloat6(ctx, usdcE)
			if err != nil {
				return err
			}
			b, err := ch.ERC20BalanceFloat6(ctx, usdc)
			if err != nil {
				return err
			}

			fmt.Printf("Wallet: %s\n", ch.Address().Hex())
			fmt.Printf("USDC.e (%s): %.6f\n", chain.USDCeAddress, bE)
			fmt.Printf("USDC   (%s): %.6f\n", chain.USDCAddress, b)
			fmt.Printf("Total: %.6f\n", bE+b)
			return nil
		},
	}
	return cmd
}

