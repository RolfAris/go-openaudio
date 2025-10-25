package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenAudio/go-openaudio/pkg/common"
	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/crypto/secp256k1"
	"github.com/cometbft/cometbft/libs/bytes"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/types"
	cmttime "github.com/cometbft/cometbft/types/time"
	"github.com/google/uuid"
)

// InitNode initializes a new OpenAudio node at cfg.CometBFT.RootDir.
// It creates CometBFT keys, a default genesis, and writes merged configuration.
func InitNode(cfg *Config, preset string, ethKey string) error {
	rootDir := cfg.CometBFT.RootDir
	configDir := filepath.Join(rootDir, "config")
	dataDir := filepath.Join(rootDir, "data")

	for _, dir := range []string{configDir, dataDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	privValKeyFile := cfg.CometBFT.PrivValidatorKeyFile()
	privValStateFile := cfg.CometBFT.PrivValidatorStateFile()

	var pv *privval.FilePV
	if common.FileExists(privValKeyFile) {
		pv = privval.LoadFilePV(privValKeyFile, privValStateFile)
	} else {
		genFilePV, err := privval.GenFilePV(privValKeyFile, privValStateFile, func() (crypto.PrivKey, error) {
			return secp256k1.GenPrivKey(), nil
		})
		if err != nil {
			return fmt.Errorf("gen file pv: %w", err)
		}
		genFilePV.Save()
		pv = privval.LoadFilePV(privValKeyFile, privValStateFile)
	}

	if _, err := p2p.LoadOrGenNodeKey(cfg.CometBFT.NodeKeyFile()); err != nil {
		return fmt.Errorf("generate node key: %w", err)
	}

	genFile := cfg.CometBFT.GenesisFile()

	hash := sha256.Sum256([]byte(uuid.NewString()))
	chainID := fmt.Sprintf("openaudio-%x", hash[:6])

	if !common.FileExists(genFile) {
		genDoc := types.GenesisDoc{
			ChainID:         chainID,
			GenesisTime:     cmttime.Now(),
			ConsensusParams: types.DefaultConsensusParams(),
		}

		genDoc.ConsensusParams.Validator.PubKeyTypes = []string{"secp256k1"}

		pubKey, err := pv.GetPubKey()
		if err != nil {
			return fmt.Errorf("get pubkey: %w", err)
		}

		genDoc.Validators = []types.GenesisValidator{{
			Address: pubKey.Address(),
			PubKey:  pubKey,
			Power:   10,
			Name:    cfg.CometBFT.Moniker,
		}}

		appState, _ := json.Marshal(DefaultGenesisData())
		genDoc.AppState = appState
		appHashSha := sha256.Sum256(appState)
		appHash := bytes.HexBytes(appHashSha[:])
		genDoc.AppHash = appHash

		if err := genDoc.SaveAs(genFile); err != nil {
			return fmt.Errorf("save genesis: %w", err)
		}
	}

	// TODO: set defaults for relevant node types
	switch strings.ToLower(preset) {
	case "seed":

	case "validator":

	case "archive":

	case "rpc":

	case "light":
	}

	if err := WriteMergedConfigFile(cfg, rootDir); err != nil {
		return fmt.Errorf("write merged config: %w", err)
	}

	return nil
}
