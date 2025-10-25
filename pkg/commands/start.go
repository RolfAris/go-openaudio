package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenAudio/go-openaudio/pkg/config"
	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func NewStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start an OpenAudio node",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := cmd.Flags().GetString("home")
			if home == "" {
				home = filepath.Join(os.Getenv("HOME"), ".openaudio")
			}

			envFile, _ := cmd.Flags().GetString("env-file")
			if envFile != "" {
				if err := godotenv.Load(envFile); err != nil {
					return fmt.Errorf("load env file: %w", err)
				}
			}

			cfgPath := filepath.Join(home, "config", "config.toml")
			cfg, err := config.Load(cfgPath, home)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			fmt.Printf("starting OpenAudio node from %s...\n", home)
			spew.Dump(cfg)

			// Return your app runner with context cancellation support
			// return app.Run(cmd.Context())
			return nil
		},
	}

	cmd.Flags().String("home", "", "Path to node home directory (default: $HOME/.openaudio)")
	cmd.Flags().String("env-file", "", "Path to .env file to load before config")
	return cmd
}
