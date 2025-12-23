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

func newWalletCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wallet",
		Short: "钱包快速排障（等价 check_balance.py + 一部分 check_all_usdc.py）",
	}
	cmd.AddCommand(newWalletSummaryCmd())
	return cmd
}

func newWalletSummaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "输出地址、MATIC、USDC、USDC.e 余额",
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

			matic, err := ch.NativeBalanceFloat18(ctx)
			if err != nil {
				return err
			}
			usdcE, err := ch.ERC20BalanceFloat6(ctx, common.HexToAddress(chain.USDCeAddress))
			if err != nil {
				return err
			}
			usdc, err := ch.ERC20BalanceFloat6(ctx, common.HexToAddress(chain.USDCAddress))
			if err != nil {
				return err
			}

			fmt.Printf("Wallet: %s\n", ch.Address().Hex())
			fmt.Printf("ChainID: %d\n", cfg.ChainID)
			fmt.Printf("MATIC: %.6f\n", matic)
			fmt.Printf("USDC.e: %.6f\n", usdcE)
			fmt.Printf("USDC: %.6f\n", usdc)
			return nil
		},
	}
	return cmd
}

