package commands

import (
	"fmt"
	"net"

	"github.com/spf13/cobra"
)

func NewStatusCmd() *cobra.Command {
	var endpoint string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Query the status of a running OpenAudio node",
		RunE: func(cmd *cobra.Command, args []string) error {
			if endpoint == "" {
				endpoint = "/tmp/openaudio.sock" // or detect from config
			}

			if _, err := net.Dial("unix", endpoint); err == nil {
				fmt.Printf("Connected to OpenAudio node via UNIX socket: %s\n", endpoint)
			} else {
				fmt.Printf("Could not connect via UNIX socket, trying ConnectRPC: %s\n", endpoint)
				// TODO: Connect to ConnectRPC endpoint here
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Node endpoint (e.g., validator1.openaudio.org or local UNIX socket)")
	return cmd
}
