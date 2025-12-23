package cli

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/cobra"

	"limitorderbot/internal/chain"
	"limitorderbot/internal/config"
)

const (
	transferSingleTopic = "0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62" // keccak TransferSingle(...)
)

func newCTFCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ctf",
		Short: "CTF (ERC1155) 工具：扫描/查余额",
	}
	cmd.AddCommand(newCTFScanCmd())
	cmd.AddCommand(newCTFBalanceCmd())
	return cmd
}

func newCTFScanCmd() *cobra.Command {
	var blocks int64
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "扫描最近 N 个区块内转入的 CTF tokenId，并输出当前余额",
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

			ctx, cancel := chain.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			latest, err := ch.EthClient().BlockNumber(ctx)
			if err != nil {
				return err
			}
			from := int64(latest) - blocks
			if from < 0 {
				from = 0
			}

			fmt.Printf("Wallet: %s\n", ch.Address().Hex())
			fmt.Printf("Scanning blocks %d to %d...\n\n", from, latest)

			logs, err := ch.EthClient().FilterLogs(ctx, ethereum.FilterQuery{
				FromBlock: big.NewInt(from),
				ToBlock:   big.NewInt(int64(latest)),
				Addresses: []common.Address{common.HexToAddress(chain.CTFAddress)},
				Topics: [][]common.Hash{
					{common.HexToHash(transferSingleTopic)},
					nil,
					nil,
					{topicAddress(ch.Address())},
				},
			})
			if err != nil {
				return err
			}
			if len(logs) == 0 {
				fmt.Println("No recent transfers found.")
				return nil
			}

			tokenIDs := map[string]struct{}{}
			for _, lg := range logs {
				id, amt, ok := decodeTransferSingle(lg)
				if !ok {
					continue
				}
				tokenIDs[id.String()] = struct{}{}
				fmt.Printf("Token ID: %s\n", id.String())
				fmt.Printf("  Amount received: %.6f shares\n", toFloat6(amt))
				fmt.Printf("  Block: %d\n\n", lg.BlockNumber)
			}

			fmt.Println(repeat("=", 60))
			fmt.Println("Checking current balances...")

			var ids []string
			for id := range tokenIDs {
				ids = append(ids, id)
			}
			sort.Strings(ids)

			total := 0.0
			for _, idStr := range ids {
				id := new(big.Int)
				id.SetString(idStr, 10)
				bal, err := ch.ERC1155BalanceOf(ctx, common.HexToAddress(chain.CTFAddress), id)
				if err != nil {
					fmt.Printf("Token %s: ERROR %v\n", idStr, err)
					continue
				}
				f := toFloat6(bal)
				fmt.Printf("Token ID: %s\n", idStr)
				fmt.Printf("  Current balance: %.6f shares\n", f)
				if bal.Sign() > 0 {
					total += f
					fmt.Printf("  Status: YOU HAVE POSITIONS ✓\n\n")
				} else {
					fmt.Printf("  Status: Already redeemed or sold\n\n")
				}
			}
			fmt.Println(repeat("=", 60))
			fmt.Printf("Total unredeemed positions: %.6f shares\n", total)
			return nil
		},
	}
	cmd.Flags().Int64Var(&blocks, "blocks", 10_000, "lookback blocks (default ~5 hours on Polygon)")
	return cmd
}

func newCTFBalanceCmd() *cobra.Command {
	var tokenID string
	cmd := &cobra.Command{
		Use:   "balance",
		Short: "查询指定 tokenId 的 CTF balanceOf",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if tokenID == "" {
				return fmt.Errorf("--token-id is required")
			}
			id := new(big.Int)
			if _, ok := id.SetString(tokenID, 10); !ok {
				return fmt.Errorf("invalid token id")
			}

			ch, err := chain.New(cfg.RPCURL, cfg.PrivateKey, cfg.ChainID)
			if err != nil {
				return err
			}
			defer ch.Close()
			ctx, cancel := chain.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			bal, err := ch.ERC1155BalanceOf(ctx, common.HexToAddress(chain.CTFAddress), id)
			if err != nil {
				return err
			}
			fmt.Printf("Wallet: %s\n", ch.Address().Hex())
			fmt.Printf("Token ID: %s\n", tokenID)
			fmt.Printf("Balance: %.6f shares\n", toFloat6(bal))
			return nil
		},
	}
	cmd.Flags().StringVar(&tokenID, "token-id", "", "ERC1155 token id (decimal)")
	return cmd
}

func topicAddress(addr common.Address) common.Hash {
	// topic is 32-byte left padded
	return common.BytesToHash(common.LeftPadBytes(addr.Bytes(), 32))
}

func decodeTransferSingle(lg types.Log) (*big.Int, *big.Int, bool) {
	// data layout: [id (32)][value (32)]
	if len(lg.Data) < 64 {
		return nil, nil, false
	}
	id := new(big.Int).SetBytes(lg.Data[:32])
	val := new(big.Int).SetBytes(lg.Data[32:64])
	return id, val, true
}

func toFloat6(v *big.Int) float64 {
	r := new(big.Rat).SetFrac(v, big.NewInt(1_000_000))
	f, _ := r.Float64()
	return f
}
