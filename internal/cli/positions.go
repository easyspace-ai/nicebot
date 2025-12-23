package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"limitorderbot/internal/chain"
	"limitorderbot/internal/config"
)

func newPositionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "positions",
		Short: "Polymarket Data API positions 工具（等价 get_positions_api.py）",
	}
	cmd.AddCommand(newPositionsListCmd())
	cmd.AddCommand(newPositionsRawCmd())
	return cmd
}

func newPositionsListCmd() *cobra.Command {
	var redeemableOnly bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出 positions（可选仅 redeemable）",
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

			ps, err := fetchPositions(ctx, ch.Address().Hex())
			if err != nil {
				return err
			}
			if redeemableOnly {
				var out []polymarketPosition
				for _, p := range ps {
					if p.Redeemable {
						out = append(out, p)
					}
				}
				ps = out
			}
			sort.Slice(ps, func(i, j int) bool { return ps[i].CurrentValue > ps[j].CurrentValue })

			fmt.Printf("Wallet: %s\n", ch.Address().Hex())
			fmt.Printf("Positions: %d\n\n", len(ps))
			for i, p := range ps {
				title := p.Title
				if title == "" {
					title = p.Slug
				}
				fmt.Printf("%3d) %.4f USD  redeemable=%v  %s  %s  outcome=%s  size=%.6f  cid=%s\n",
					i+1, p.CurrentValue, p.Redeemable, title, p.Slug, p.Outcome, p.Size, p.ConditionID,
				)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&redeemableOnly, "redeemable-only", false, "仅显示 redeemable=true")
	return cmd
}

func newPositionsRawCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "raw",
		Short: "输出 Data API 原始 JSON",
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

			ps, err := fetchPositions(ctx, ch.Address().Hex())
			if err != nil {
				return err
			}
			b, _ := json.MarshalIndent(ps, "", "  ")
			fmt.Println(string(b))
			return nil
		},
	}
	return cmd
}

