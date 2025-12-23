package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/cobra"

	"limitorderbot/internal/chain"
	"limitorderbot/internal/config"
)

func newTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tx",
		Short: "交易/回执解析工具（等价 get_token_ids_from_tx.py）",
	}
	cmd.AddCommand(newTxTokenIDsCmd())
	return cmd
}

func newTxTokenIDsCmd() *cobra.Command {
	var txHash string
	var onlyIncoming bool
	cmd := &cobra.Command{
		Use:   "token-ids",
		Short: "从交易回执里解析 CTF TransferSingle 的 tokenId/amount",
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

			h := common.HexToHash(strings.TrimSpace(txHash))
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			rcpt, err := ch.EthClient().TransactionReceipt(ctx, h)
			if err != nil {
				return err
			}

			wallet := ch.Address()
			ctfAddr := common.HexToAddress(chain.CTFAddress)

			fmt.Printf("Wallet: %s\n", wallet.Hex())
			fmt.Printf("Tx: %s\n", h.Hex())
			fmt.Printf("Status: %d, Logs: %d\n\n", rcpt.Status, len(rcpt.Logs))

			found := 0
			for _, lg := range rcpt.Logs {
				if lg.Address != ctfAddr {
					continue
				}
				if len(lg.Topics) == 0 || lg.Topics[0].Hex() != transferSingleTopic {
					continue
				}
				if onlyIncoming && !isTransferSingleToWallet(lg, wallet) {
					continue
				}
				id, amt, ok := decodeTransferSingle(*lg)
				if !ok {
					continue
				}
				found++
				fmt.Printf("TransferSingle token_id=%s amount=%s (%.6f shares)\n", id.String(), amt.String(), toFloat6(amt))
			}
			if found == 0 {
				fmt.Println("No matching TransferSingle logs found.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&txHash, "tx", "", "transaction hash (0x...)")
	cmd.Flags().BoolVar(&onlyIncoming, "only-incoming", true, "只输出转入当前钱包的 token")
	_ = cmd.MarkFlagRequired("tx")
	return cmd
}

func isTransferSingleToWallet(lg *types.Log, wallet common.Address) bool {
	// TransferSingle(operator indexed, from indexed, to indexed, id, value)
	// topics: [sig, operator, from, to]
	if lg == nil || len(lg.Topics) < 4 {
		return false
	}
	return lg.Topics[3] == topicAddress(wallet)
}

