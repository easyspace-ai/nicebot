package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"limitorderbot/internal/chain"
	"limitorderbot/internal/config"
)

type polymarketPosition struct {
	ConditionID  string  `json:"conditionId"`
	Title        string  `json:"title"`
	Slug         string  `json:"slug"`
	Outcome      string  `json:"outcome"`
	Size         float64 `json:"size"`
	CurPrice     float64 `json:"curPrice"`
	CurrentValue float64 `json:"currentValue"`
	Redeemable   bool    `json:"redeemable"`
}

func newRedeemAllCmd() *cobra.Command {
	var yes bool
	var limit int
	cmd := &cobra.Command{
		Use:   "redeem-all",
		Short: "列出并批量赎回所有 redeemable markets（等价 redeem_positions.py/auto_redeem.py）",
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

			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()

			positions, err := fetchPositions(ctx, ch.Address().Hex())
			if err != nil {
				return err
			}
			byCID := map[string][]polymarketPosition{}
			for _, p := range positions {
				if !p.Redeemable {
					continue
				}
				byCID[p.ConditionID] = append(byCID[p.ConditionID], p)
			}

			if len(byCID) == 0 {
				fmt.Println("No redeemable positions found.")
				return nil
			}

			type item struct {
				cid   string
				title string
				value float64
				count int
			}
			var items []item
			for cid, ps := range byCID {
				title := ps[0].Title
				if strings.TrimSpace(title) == "" {
					title = ps[0].Slug
				}
				sum := 0.0
				for _, p := range ps {
					sum += p.CurrentValue
				}
				items = append(items, item{cid: cid, title: title, value: sum, count: len(ps)})
			}
			sort.Slice(items, func(i, j int) bool { return items[i].value > items[j].value })
			if limit > 0 && limit < len(items) {
				items = items[:limit]
			}

			fmt.Printf("Wallet: %s\n\n", ch.Address().Hex())
			fmt.Printf("Redeemable markets: %d\n\n", len(items))
			total := 0.0
			for i, it := range items {
				total += it.value
				fmt.Printf("%2d) %.4f USD  (%d pos)  %s  cid=%s\n", i+1, it.value, it.count, it.title, it.cid)
			}
			fmt.Printf("\nEstimated total value: %.4f USD\n", total)

			if !yes {
				fmt.Print("\nProceed to redeem all listed conditionIds? Type 'yes' to continue: ")
				in := bufio.NewReader(os.Stdin)
				line, _ := in.ReadString('\n')
				if strings.TrimSpace(strings.ToLower(line)) != "yes" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			fmt.Println("\nRedeeming...")
			redeemed := 0
			for _, it := range items {
				cond, err := chain.ConditionIDFromHex(it.cid)
				if err != nil {
					fmt.Printf("skip cid=%s: %v\n", it.cid, err)
					continue
				}
				tx, err := ch.RedeemPositions(ctx, cond)
				if err != nil {
					fmt.Printf("fail cid=%s: %v\n", it.cid, err)
					continue
				}
				redeemed++
				fmt.Printf("ok cid=%s tx=%s\n", it.cid, tx.Hex())
			}
			fmt.Printf("\nDone. Redeemed %d market(s).\n", redeemed)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "跳过确认直接执行")
	cmd.Flags().IntVar(&limit, "limit", 0, "最多处理前 N 个（按 currentValue 降序）")
	return cmd
}

func newClaimWinningsCmd() *cobra.Command {
	// Python 的 claim_winnings.py 在实际使用中等价于“批量 redeem 可赎回头寸”；
	// 这里提供别名命令，保持工具链 1:1 可用性。
	cmd := newRedeemAllCmd()
	cmd.Use = "claim-winnings"
	cmd.Short = "别名：redeem-all（等价 claim_winnings.py）"
	return cmd
}

func fetchPositions(ctx context.Context, wallet string) ([]polymarketPosition, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://data-api.polymarket.com/positions?user="+wallet, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("positions api status=%d", resp.StatusCode)
	}
	var positions []polymarketPosition
	if err := json.NewDecoder(resp.Body).Decode(&positions); err != nil {
		return nil, err
	}
	return positions, nil
}

