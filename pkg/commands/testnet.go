package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/OpenAudio/go-openaudio/pkg/config"
)

func NewTestnetCmd() *cobra.Command {
	var (
		validators int
		seeds      int
		archives   int
		rpcs       int
		light      int
		rootDir    string
		chainID    string
	)

	cmd := &cobra.Command{
		Use:   "testnet",
		Short: "Set up a local multi-validator testnet",
		Long: `Generate a testnet with multiple OpenAudio nodes.

This command creates a complete testnet with configurable numbers of each node type.
All nodes share a common genesis file, and validators are automatically configured
as persistent peers for docker networking.

Example:
  openaudio testnet --root ./tmp/testnet --validators 3 --seeds 1 --rpcs 2`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if rootDir == "" {
				return fmt.Errorf("--root flag is required")
			}

			// Collect counts
			counts := map[string]int{
				"validator": validators,
				"seed":      seeds,
				"archive":   archives,
				"rpc":       rpcs,
				"light":     light,
			}

			// Validate at least one node
			totalNodes := validators + seeds + archives + rpcs + light
			if totalNodes == 0 {
				return fmt.Errorf("at least one node is required")
			}

			fmt.Printf("Generating testnet with %d total nodes:\n", totalNodes)
			if validators > 0 {
				fmt.Printf("  - Validators: %d\n", validators)
			}
			if seeds > 0 {
				fmt.Printf("  - Seeds: %d\n", seeds)
			}
			if archives > 0 {
				fmt.Printf("  - Archives: %d\n", archives)
			}
			if rpcs > 0 {
				fmt.Printf("  - RPCs: %d\n", rpcs)
			}
			if light > 0 {
				fmt.Printf("  - Light clients: %d\n", light)
			}

			// Generate testnet
			if err := config.GenerateTestnet(rootDir, chainID, counts); err != nil {
				return fmt.Errorf("generate testnet: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&validators, "validators", "v", 3, "Number of validator nodes")
	cmd.Flags().IntVarP(&seeds, "seeds", "s", 0, "Number of seed nodes")
	cmd.Flags().IntVarP(&archives, "archives", "a", 0, "Number of archive nodes")
	cmd.Flags().IntVarP(&rpcs, "rpcs", "r", 0, "Number of rpc nodes")
	cmd.Flags().IntVarP(&light, "light", "l", 0, "Number of light client nodes")
	cmd.Flags().StringVar(&rootDir, "root", "", "Root directory for all node homes (required)")
	cmd.Flags().StringVarP(&chainID, "chain-id", "c", "", "Chain ID (auto-generated if not provided)")

	return cmd
}
