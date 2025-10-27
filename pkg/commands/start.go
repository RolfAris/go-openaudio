package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenAudio/go-openaudio/pkg/app"
	"github.com/OpenAudio/go-openaudio/pkg/config"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
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
			_, err := config.Load(cfgPath, home)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			logger, _ := zap.NewProduction()
			app := app.NewApp(cmd.Context(), logger)
			err = app.Run()
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		},
	}

	cmd.Flags().String("home", "", "Path to node home directory (default: $HOME/.openaudio)")
	cmd.Flags().String("env-file", "", "Path to .env file to load before config")
	return cmd
}
