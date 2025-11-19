package commands

import (
	"context"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "openaudio",
		Short: "OpenAudio node and management CLI",
		Long: `OpenAudio is a CometBFT-based blockchain node and toolchain
for the Open Audio Protocol (OAP).`,
	}

	root.PersistentFlags().String("home", "", "Directory for config and data (default: $HOME/.openaudio)")

	// Register subcommands
	root.AddCommand(
		NewInitCmd(),
		NewStartCmd(),
		NewStatusCmd(),
		NewTestnetCmd(),
	)

	return root
}

func Execute(ctx context.Context) error {
	return NewRootCmd().ExecuteContext(ctx)
}
