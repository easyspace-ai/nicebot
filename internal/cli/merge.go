package cli

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/spf13/cobra"

	"limitorderbot/internal/chain"
	"limitorderbot/internal/config"
)

func newMergeCmd() *cobra.Command {
	var conditionID string
	var amount float64
	cmd := &cobra.Command{
		Use:   "merge",
		Short: "按 condition_id 调用 CTF.mergePositions 合并 YES/NO 回 USDC",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if conditionID == "" {
				return fmt.Errorf("--condition-id is required (0x...)")
			}
			if amount <= 0 {
				return fmt.Errorf("--amount must be > 0 (单位: sets / USDC)")
			}
			cid, err := chain.ConditionIDFromHex(conditionID)
			if err != nil {
				return err
			}

			amountUSDC6 := big.NewInt(int64(amount * 1e6))
			ch, err := chain.New(cfg.RPCURL, cfg.PrivateKey, cfg.ChainID)
			if err != nil {
				return err
			}
			defer ch.Close()

			ctx, cancel := chain.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			tx, err := ch.MergePositions(ctx, cid, amountUSDC6)
			if err != nil {
				return err
			}
			fmt.Printf("✓ Merge tx sent: %s\n", tx.Hex())
			return nil
		},
	}
	cmd.Flags().StringVar(&conditionID, "condition-id", "", "condition id (0x...)")
	cmd.Flags().Float64Var(&amount, "amount", 0, "merge amount (float, sets; will be scaled by 1e6)")
	return cmd
}
