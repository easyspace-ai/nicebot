package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"limitorderbot/internal/clob"
	"limitorderbot/internal/config"
	"limitorderbot/internal/gamma"
	"limitorderbot/internal/models"
)

func newCLOBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clob",
		Short: "Polymarket CLOB 工具：查单/更新 L2 allowance/测试下单",
	}
	cmd.AddCommand(newCLOBOpenOrdersCmd())
	cmd.AddCommand(newCLOBUpdateL2BalanceCmd())
	cmd.AddCommand(newCLOBPlaceTestCmd())
	return cmd
}

func newCLOBOpenOrdersCmd() *cobra.Command {
	var market string
	var assetID string
	cmd := &cobra.Command{
		Use:   "open-orders",
		Short: "查询当前钱包的 open orders（等价 check_open_orders.py）",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cc, err := clob.NewClient(cfg.ClobAPIURL, cfg.ChainID, cfg.PrivateKey, cfg.SignatureType, cfg.FunderAddress)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			creds, err := cc.CreateOrDeriveAPICreds(ctx, 0)
			if err != nil {
				return err
			}
			cc.SetCreds(creds)

			params := &clob.OpenOrderParams{Market: market, AssetID: assetID}
			orders, err := cc.GetOrders(ctx, params)
			if err != nil {
				return err
			}
			fmt.Printf("Wallet: %s\n\n", cc.Address())
			if len(orders) == 0 {
				fmt.Println("No open orders found.")
				return nil
			}
			fmt.Printf("Found %d open order(s):\n\n", len(orders))
			for _, o := range orders {
				fmt.Printf("Order: %v\n\n", o)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&market, "market", "", "condition id filter (market)")
	cmd.Flags().StringVar(&assetID, "asset-id", "", "token id filter (asset_id)")
	return cmd
}

func newCLOBUpdateL2BalanceCmd() *cobra.Command {
	var assetType string
	var tokenID string
	var signatureType int
	cmd := &cobra.Command{
		Use:   "update-l2-balance",
		Short: "调用 /balance-allowance/update 并输出 /balance-allowance（等价 update_l2_balance.py）",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cc, err := clob.NewClient(cfg.ClobAPIURL, cfg.ChainID, cfg.PrivateKey, cfg.SignatureType, cfg.FunderAddress)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			creds, err := cc.CreateOrDeriveAPICreds(ctx, 0)
			if err != nil {
				return err
			}
			cc.SetCreds(creds)

			if assetType == "" {
				assetType = "COLLATERAL"
			}
			params := &clob.BalanceAllowanceParams{
				AssetType:     strings.ToUpper(assetType),
				TokenID:       tokenID,
				SignatureType: signatureType,
			}

			fmt.Println("Updating balance allowance...")
			upd, err := cc.UpdateBalanceAllowance(ctx, params)
			if err != nil {
				return err
			}
			fmt.Printf("Result: %v\n\n", upd)

			fmt.Println("Fetching balance allowance...")
			cur, err := cc.GetBalanceAllowance(ctx, params)
			if err != nil {
				return err
			}
			fmt.Printf("Balance info: %v\n", cur)
			return nil
		},
	}
	cmd.Flags().StringVar(&assetType, "asset-type", "COLLATERAL", "COLLATERAL | CONDITIONAL")
	cmd.Flags().StringVar(&tokenID, "token-id", "", "conditional token id (optional)")
	cmd.Flags().IntVar(&signatureType, "signature-type", 0, "override signature_type (0/1/2). Default uses SIGNATURE_TYPE")
	return cmd
}

func newCLOBPlaceTestCmd() *cobra.Command {
	var price float64
	var size float64
	var yes bool
	cmd := &cobra.Command{
		Use:   "place-test",
		Short: "在第一个可用 BTC 15m 市场下 2 笔测试单（等价 place_test_order.py/test_small_order.py）",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			disc := gamma.New(cfg.GammaAPIBaseURL)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			markets, err := disc.DiscoverBTC15mMarkets(ctx)
			if err != nil {
				return err
			}
			if len(markets) == 0 {
				return fmt.Errorf("no BTC 15m markets found")
			}
			m := markets[0]
			fmt.Printf("Using market: %s\n", m.MarketSlug)
			fmt.Printf("Price: %.2f, Size: %.2f shares\n", price, size)
			if !yes {
				fmt.Println("Dry-run: add --yes to actually place orders.")
				return nil
			}

			cc, err := clob.NewClient(cfg.ClobAPIURL, cfg.ChainID, cfg.PrivateKey, cfg.SignatureType, cfg.FunderAddress)
			if err != nil {
				return err
			}
			creds, err := cc.CreateOrDeriveAPICreds(ctx, 0)
			if err != nil {
				return err
			}
			cc.SetCreds(creds)

			yesOut, noOut := inferYesNoOutcomes(m.Outcomes)
			if yesOut == nil || noOut == nil {
				return fmt.Errorf("could not infer YES/NO outcomes from market outcomes")
			}

			placed := 0
			for _, out := range []models.Outcome{*yesOut, *noOut} {
				args := clob.OrderArgs{
					TokenID:    out.TokenID,
					Price:      price,
					Size:       size,
					Side:       clob.OrderSideBuy,
					FeeRateBps: 0,
					Nonce:      0,
					Expiration: 0,
				}
				signed, _, err := cc.CreateOrder(ctx, args, nil, nil)
				if err != nil {
					return err
				}
				resp, err := cc.PostOrder(ctx, signed, clob.OrderTypeGTC)
				if err != nil {
					return err
				}
				placed++
				fmt.Printf("Placed BUY %s token_id=%s resp=%v\n", out.Outcome, out.TokenID, resp)
			}
			fmt.Printf("\nPlaced %d order(s)\n", placed)
			return nil
		},
	}
	cmd.Flags().Float64Var(&price, "price", 0.49, "limit price")
	cmd.Flags().Float64Var(&size, "size", 10.0, "shares per order")
	cmd.Flags().BoolVar(&yes, "yes", false, "确认下单")
	return cmd
}

func inferYesNoOutcomes(outs []models.Outcome) (*models.Outcome, *models.Outcome) {
	var y, n *models.Outcome
	for i := range outs {
		u := strings.ToUpper(strings.TrimSpace(outs[i].Outcome))
		switch u {
		case "YES", "UP", "TRUE":
			if y == nil {
				y = &outs[i]
			}
		case "NO", "DOWN", "FALSE":
			if n == nil {
				n = &outs[i]
			}
		}
	}
	return y, n
}

