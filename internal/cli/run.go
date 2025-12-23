package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"limitorderbot/internal/bot"
	"limitorderbot/internal/config"
	"limitorderbot/internal/dashboard"
	"limitorderbot/internal/logging"
)

func newRunCmd() *cobra.Command {
	var mode string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "运行 bot / dashboard / both",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			b, err := bot.New(cfg)
			if err != nil {
				return err
			}
			defer b.Close()

			ctx, cancel := signalContext()
			defer cancel()

			if err := b.Start(ctx); err != nil {
				return err
			}

			switch mode {
			case "bot":
				return runBotLoop(ctx, b, cfg)
			case "dashboard", "both":
				// Start bot loop in background, then serve dashboard.
				go func() {
					_ = runBotLoop(ctx, b, cfg)
				}()
				s, err := dashboard.New(cfg, b)
				if err != nil {
					return err
				}
				logging.Logger().Printf("Starting dashboard on %s:%d\n", cfg.DashboardHost, cfg.DashboardPort)
				err = s.Run(ctx)
				if err != nil && err.Error() != "http: Server closed" {
					return err
				}
				return nil
			default:
				return fmt.Errorf("invalid --mode: %s (bot|dashboard|both)", mode)
			}
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "both", "运行模式: bot|dashboard|both")
	return cmd
}

func runBotLoop(ctx context.Context, b *bot.Bot, cfg config.Config) error {
	log := logging.Logger()
	ticker := time.NewTicker(time.Duration(cfg.CheckIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutdown requested")
			b.Stop()
			return nil
		default:
		}

		loopCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.CheckIntervalSeconds)*time.Second)
		b.RunOnce(loopCtx)
		cancel()

		log.Printf("Sleeping for %d seconds...\n", cfg.CheckIntervalSeconds)
		select {
		case <-ctx.Done():
			b.Stop()
			return nil
		case <-ticker.C:
		}
	}
}

func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()
	return ctx, cancel
}
