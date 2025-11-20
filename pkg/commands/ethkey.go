package commands

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenAudio/go-openaudio/pkg/common"
	"github.com/OpenAudio/go-openaudio/pkg/config"
	cmtcrypto "github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/cometbft/cometbft/privval"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
)

// NewImportEthKeyCmd imports an Ethereum private key as the validator key.
func NewImportEthKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import-eth-key",
		Short: "Import an Ethereum private key as the validator key",
		Example: `
openaudio import-eth-key --key 0xabc123... --home ~/.openaudio
		`,
		RunE: func(cmd *cobra.Command, args []string) error {
			keyHex, _ := cmd.Flags().GetString("key")
			home, _ := cmd.Flags().GetString("home")

			if keyHex == "" {
				return fmt.Errorf("missing --key argument")
			}
			if home == "" {
				home = os.ExpandEnv(filepath.Join("$HOME", ".openaudio"))
			}

			if err := os.MkdirAll(filepath.Join(home, "config"), 0o755); err != nil {
				return fmt.Errorf("create config dir: %w", err)
			}
			if err := os.MkdirAll(filepath.Join(home, "data"), 0o755); err != nil {
				return fmt.Errorf("create data dir: %w", err)
			}

			keyHex = strings.TrimPrefix(keyHex, "0x")
			keyBytes, err := hex.DecodeString(keyHex)
			if err != nil {
				return fmt.Errorf("invalid private key: %w", err)
			}

			privECDSA, err := ethcrypto.ToECDSA(keyBytes)
			if err != nil {
				return fmt.Errorf("parse private key: %w", err)
			}

			privValKeyFile := filepath.Join(home, "config", "priv_validator_key.json")
			privValStateFile := filepath.Join(home, "data", "priv_validator_state.json")

			if common.FileExists(privValKeyFile) {
				return fmt.Errorf("validator key already exists at %s", privValKeyFile)
			}

			var privKey secp256k1.PrivKey
			copy(privKey[:], ethcrypto.FromECDSA(privECDSA)[:32])

			pv, err := privval.GenFilePV(privValKeyFile, privValStateFile, func() (cmtcrypto.PrivKey, error) {
				return privKey, nil
			})
			if err != nil {
				return fmt.Errorf("gen file pv: %w", err)
			}

			pv.Save()

			// Add eth_address to priv_validator_key.json
			if err := config.AddEthAddressToValidatorKey(privValKeyFile, pv); err != nil {
				return fmt.Errorf("add eth address to validator key: %w", err)
			}

			addr := common.PubKeyToAddress(&privECDSA.PublicKey)
			cmd.Printf("Imported Ethereum key as validator key\nAddress: %s\nSaved to: %s\n", addr, privValKeyFile)
			return nil
		},
	}

	cmd.Flags().String("key", "", "Ethereum private key (hex string, with or without 0x prefix)")
	cmd.Flags().String("home", "", "node home directory (default: $HOME/.openaudio)")
	return cmd
}
