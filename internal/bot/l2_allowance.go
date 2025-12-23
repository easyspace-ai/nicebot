package bot

import (
	"context"
	"fmt"
	"strings"

	"limitorderbot/internal/clob"
	"limitorderbot/internal/logging"
)

// updateL2BalanceAllowanceBestEffort mirrors python OrderManager._set_allowances():
// it attempts to call /balance-allowance/update for collateral (USDC) and then reads /balance-allowance.
// Errors are logged as warnings and do not fail startup.
func (b *Bot) updateL2BalanceAllowanceBestEffort(ctx context.Context) {
	if b.clob == nil {
		return
	}
	log := logging.Logger()

	params := &clob.BalanceAllowanceParams{
		AssetType: "COLLATERAL",
		// TokenID empty for collateral
		// SignatureType 0 -> use default (from SIGNATURE_TYPE)
	}

	log.Println("Updating L2 balance allowance (COLLATERAL/USDC)...")
	upd, err := b.clob.UpdateBalanceAllowance(ctx, params)
	if err != nil {
		// Match python: warn and continue.
		log.Printf("WARNING: Could not update L2 balance allowance: %v\n", err)
	} else {
		log.Printf("L2 balance allowance update result: %v\n", upd)
	}

	cur, err := b.clob.GetBalanceAllowance(ctx, params)
	if err != nil {
		log.Printf("WARNING: Could not fetch L2 balance allowance: %v\n", err)
		return
	}
	// Normalize a bit for readability without assuming schema.
	if m, ok := cur["balance_allowance"].(map[string]any); ok && m != nil {
		log.Printf("L2 balance_allowance: %s\n", fmt.Sprintf("%v", m))
		return
	}
	// fallback: print keys only
	keys := []string{}
	for k := range cur {
		keys = append(keys, k)
	}
	if len(keys) > 0 {
		log.Printf("L2 balance_allowance keys: %s\n", strings.Join(keys, ", "))
	}
}

