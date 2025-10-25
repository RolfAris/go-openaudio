package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start an OpenAudio node",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := cmd.Flags().GetString("home")
			if home == "" {
				home = "$HOME/.openaudio"
			}

			fmt.Printf("🟢 Starting OpenAudio node from %s...\n", home)
			// TODO: hook into your node launcher logic here
			return nil
		},
	}
	return cmd
}
