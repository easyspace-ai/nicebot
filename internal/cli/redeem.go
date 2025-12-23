package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"limitorderbot/internal/chain"
	"limitorderbot/internal/config"
)

func newRedeemCmd() *cobra.Command {
	var conditionID string
	cmd := &cobra.Command{
		Use:   "redeem",
		Short: "按 condition_id 调用 CTF.redeemPositions 赎回",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if conditionID == "" {
				return fmt.Errorf("--condition-id is required (0x...)")
			}
			cid, err := chain.ConditionIDFromHex(conditionID)
			if err != nil {
				return err
			}
			ch, err := chain.New(cfg.RPCURL, cfg.PrivateKey, cfg.ChainID)
			if err != nil {
				return err
			}
			defer ch.Close()

			ctx, cancel := chain.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			tx, err := ch.RedeemPositions(ctx, cid)
			if err != nil {
				return err
			}
			fmt.Printf("✓ Redeem tx sent: %s\n", tx.Hex())
			return nil
		},
	}
	cmd.Flags().StringVar(&conditionID, "condition-id", "", "condition id (0x...)")
	return cmd
}
