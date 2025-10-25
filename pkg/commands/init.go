package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/OpenAudio/go-openaudio/pkg/config"
)

func NewInitCmd() *cobra.Command {
	var (
		preset string
		ethKey string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize an OpenAudio node (CometBFT + OpenAudio config)",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := cmd.Flags().GetString("home")
			if home == "" {
				home = os.ExpandEnv(filepath.Join("$HOME", ".openaudio"))
			}

			if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
				return fmt.Errorf("create config dir: %w", err)
			}

			cfg := config.DefaultConfig()
			cfg.CometBFT.SetRoot(home)

			if err := config.InitNode(cfg, preset, ethKey); err != nil {
				return fmt.Errorf("init node: %w", err)
			}

			fmt.Printf("OpenAudio node initialized at %s (preset=%s)\n", home, preset)
			return nil
		},
	}

	cmd.Flags().StringVar(&preset, "preset", "validator", "Node preset: seed|validator|archive|rpc|light")
	cmd.Flags().StringVar(&ethKey, "eth-key", "", "Ethereum delegate private key (optional)")
	return cmd
}
