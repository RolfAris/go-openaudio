package main

import (
	"os"
	"path/filepath"

	cmtcmd "github.com/cometbft/cometbft/cmd/cometbft/commands"
	cmtdebug "github.com/cometbft/cometbft/cmd/cometbft/commands/debug"
	cmtcli "github.com/cometbft/cometbft/libs/cli"
	cmtnode "github.com/cometbft/cometbft/node"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "openaudio",
		Short: "OpenAudio node and toolchain CLI",
	}

	root.AddCommand(newCometSubtree())

	home := os.ExpandEnv(filepath.Join("$HOME", ".openaudio"))
	ensureDir(filepath.Join(home, "config"))

	// use our own env prefix and home dir
	cmd := cmtcli.PrepareBaseCmd(root, "OAP", home)
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}

func ensureDir(path string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		panic(err)
	}
}

func newCometSubtree() *cobra.Command {
	cmtRoot := *cmtcmd.RootCmd
	cmtRoot.Use = "cometbft"
	cmtRoot.Short = "CometBFT node utilities"
	cmtRoot.Aliases = []string{"comet"}

	cmtRoot.AddCommand(
		cmtcmd.GenValidatorCmd,
		cmtcmd.InitFilesCmd,
		cmtcmd.LightCmd,
		cmtcmd.ResetAllCmd,
		cmtcmd.ResetPrivValidatorCmd,
		cmtcmd.ResetStateCmd,
		cmtcmd.ShowValidatorCmd,
		cmtcmd.TestnetFilesCmd,
		cmtcmd.ShowNodeIDCmd,
		cmtcmd.GenNodeKeyCmd,
		cmtcmd.VersionCmd,
		cmtcmd.RollbackStateCmd,
		cmtcmd.CompactGoLevelDBCmd,
		cmtcmd.InspectCmd,
		cmtdebug.DebugCmd,
		cmtcli.NewCompletionCmd(&cmtRoot, true),
		cmtcmd.NewRunNodeCmd(cmtnode.DefaultNewNode),
	)

	return &cmtRoot
}
