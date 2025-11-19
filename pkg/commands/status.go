package commands

import (
	"encoding/json"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/OpenAudio/go-openaudio/pkg/sdk"
	"github.com/spf13/cobra"
)

func NewStatusCmd() *cobra.Command {
	var endpoint string
	var socket string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Query the status of a running OpenAudio node",
		RunE: func(cmd *cobra.Command, args []string) error {
			if endpoint != "" && socket != "" {
				return errors.New("cannot specify both --endpoint and --socket")
			}

			var oap *sdk.OpenAudioSDK
			switch {
			case endpoint != "":
				oap = sdk.NewOpenAudioSDK(endpoint)
			case socket != "":
				oap = sdk.NewOpenAudioSDK(socket)
			default:
				return errors.New("must specify either --endpoint or --socket")
			}

			res, err := oap.Core.GetStatus(cmd.Context(), &connect.Request[v1.GetStatusRequest]{})
			if err != nil {
				return err
			}

			msg := res.Msg
			data, err := json.MarshalIndent(msg, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal response: %w", err)
			}

			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}

	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Node endpoint (e.g. https://validator1.openaudio.org)")
	cmd.Flags().StringVar(&socket, "socket", "", "Local UNIX socket path to the node")

	return cmd
}
