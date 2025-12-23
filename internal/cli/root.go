package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func Execute() int {
	root := &cobra.Command{
		Use:   "polymarket-bot",
		Short: "Polymarket Limit Order Bot (Go port)",
	}

	root.AddCommand(newRunCmd())
	root.AddCommand(newCheckConfigCmd())
	root.AddCommand(newTestConnectionCmd())
	root.AddCommand(newRedeemCmd())
	root.AddCommand(newMergeCmd())
	root.AddCommand(newAllowancesCmd())
	root.AddCommand(newCTFCmd())

	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
