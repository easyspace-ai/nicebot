package cli

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"limitorderbot/internal/chain"
	"limitorderbot/internal/config"
)

var spenderList = []struct {
	Addr string
	Name string
}{
	{"0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E", "CTF Exchange"},
	{"0xC5d563A36AE78145C45a50134d48A1215220f80a", "Neg Risk CTF Exchange"},
	{"0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296", "Neg Risk Adapter"},
}

func newAllowancesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "allowances",
		Short: "检查/设置 Polymarket 交易所需 allowances",
	}
	cmd.AddCommand(newAllowancesCheckCmd())
	cmd.AddCommand(newAllowancesSetAllCmd())
	return cmd
}

func newAllowancesCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "检查 USDC allowance + CTF approval",
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

			ctx, cancel := chain.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			fmt.Printf("Wallet: %s\n", ch.Address().Hex())
			allGood := true
			usdc := common.HexToAddress(chain.USDCeAddress)
			ctf := common.HexToAddress(chain.CTFAddress)

			for _, s := range spenderList {
				sp := common.HexToAddress(s.Addr)
				allow, err := ch.ERC20Allowance(ctx, usdc, sp)
				if err != nil {
					return err
				}
				allowUSDC := new(big.Rat).SetFrac(allow, big.NewInt(1_000_000))
				allowF, _ := allowUSDC.Float64()

				approved, err := ch.ERC1155IsApprovedForAll(ctx, ctf, sp)
				if err != nil {
					return err
				}
				fmt.Printf("\n%s:\n", s.Name)
				fmt.Printf("  Address: %s\n", s.Addr)
				fmt.Printf("  USDC Allowance: $%.2f", allowF)
				if allow.Sign() > 0 {
					fmt.Printf(" [OK]\n")
				} else {
					fmt.Printf(" [NOT SET]\n")
					allGood = false
				}
				fmt.Printf("  CTF Approved: %v", approved)
				if approved {
					fmt.Printf(" [OK]\n")
				} else {
					fmt.Printf(" [NOT SET]\n")
					allGood = false
				}
			}

			fmt.Println("\n" + repeat("=", 70))
			if allGood {
				fmt.Println("[OK] All allowances are properly set!")
			} else {
				fmt.Println("[ERROR] Some allowances are missing - run `allowances set-all`")
			}
			return nil
		},
	}
}

func newAllowancesSetAllCmd() *cobra.Command {
	var approveUSDC float64
	cmd := &cobra.Command{
		Use:   "set-all",
		Short: "为三个 spender 设置 USDC approve + CTF setApprovalForAll",
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

			ctx, cancel := chain.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			fmt.Printf("Wallet: %s\n", ch.Address().Hex())

			amount := big.NewInt(int64(approveUSDC * 1_000_000))
			if approveUSDC <= 0 {
				amount = big.NewInt(1_000_000 * 1_000_000) // 1,000,000 USDC
			}

			for _, s := range spenderList {
				sp := common.HexToAddress(s.Addr)
				fmt.Printf("\nProcessing %s (%s)\n", s.Name, s.Addr)

				tx1, err := ch.ApproveUSDC(ctx, sp, amount)
				if err != nil {
					fmt.Printf("  USDC approve ERROR: %v\n", err)
				} else {
					fmt.Printf("  USDC approve TX: %s\n", tx1.Hex())
				}

				tx2, err := ch.SetCTFApprovalForAll(ctx, sp, true)
				if err != nil {
					fmt.Printf("  CTF approval ERROR: %v\n", err)
				} else {
					fmt.Printf("  CTF approval TX: %s\n", tx2.Hex())
				}
			}

			fmt.Println("\nDone.")
			return nil
		},
	}
	cmd.Flags().Float64Var(&approveUSDC, "approve-usdc", 0, "approve amount in USDC (default 1,000,000)")
	return cmd
}
