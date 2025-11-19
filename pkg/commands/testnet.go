package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewTestnetCmd() *cobra.Command {
	var validators int

	cmd := &cobra.Command{
		Use:   "testnet",
		Short: "Set up a local multi-validator testnet",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("🚧 Generating testnet with %d validators...\n", validators)
			// TODO: mirror CometBFT testnet logic here
			return nil
		},
	}

	cmd.Flags().IntVarP(&validators, "validators", "v", 4, "Number of validators to create")
	return cmd
}
