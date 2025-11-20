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
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
)

// CreateNodeDirectories creates the necessary directories for a node.
func createNodeDirectories(rootDir string) error {
	configDir := filepath.Join(rootDir, "config")
	dataDir := filepath.Join(rootDir, "data")

	for _, dir := range []string{configDir, dataDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	return nil
}

// generateNodeKeys creates validator and node keys for a node.
// Returns the FilePV and NodeKey, or an error.
func generateNodeKeys(privValKeyFile, privValStateFile, nodeKeyFile string) (*privval.FilePV, *p2p.NodeKey, error) {
	var pv *privval.FilePV
	if common.FileExists(privValKeyFile) {
		pv = privval.LoadFilePV(privValKeyFile, privValStateFile)
	} else {
		genFilePV, err := privval.GenFilePV(privValKeyFile, privValStateFile, func() (crypto.PrivKey, error) {
			return secp256k1.GenPrivKey(), nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("gen file pv: %w", err)
		}
		genFilePV.Save()
		pv = privval.LoadFilePV(privValKeyFile, privValStateFile)
	}

	nodeKey, err := p2p.LoadOrGenNodeKey(nodeKeyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("generate node key: %w", err)
	}

	return pv, nodeKey, nil
}

// ValidatorKeyJSON preserves the CometBFT structure with eth_address added at the top
type ValidatorKeyJSON struct {
	EthAddress string          `json:"eth_address"`
	Address    string          `json:"address"`
	PubKey     json.RawMessage `json:"pub_key"`
	PrivKey    json.RawMessage `json:"priv_key"`
}

// AddEthAddressToValidatorKey adds an eth_address field at the top of priv_validator_key.json
func AddEthAddressToValidatorKey(privValKeyFile string, pv *privval.FilePV) error {
	// Get the public key to compute Ethereum address
	pubKey, err := pv.GetPubKey()
	if err != nil {
		return fmt.Errorf("get pubkey: %w", err)
	}

	// Compute Ethereum address
	pubKeyBytes := pubKey.Bytes()
	ecdsaPubKey, err := ethcrypto.DecompressPubkey(pubKeyBytes)
	if err != nil {
		return fmt.Errorf("decompress pubkey: %w", err)
	}
	ethAddr := common.PubKeyToAddress(ecdsaPubKey)

	// Read the original file to preserve the nested structure
	originalData, err := os.ReadFile(privValKeyFile)
	if err != nil {
		return fmt.Errorf("read validator key file: %w", err)
	}

	// Parse the original to extract the properly formatted pub_key and priv_key
	var original struct {
		Address string          `json:"address"`
		PubKey  json.RawMessage `json:"pub_key"`
		PrivKey json.RawMessage `json:"priv_key"`
	}
	if err := json.Unmarshal(originalData, &original); err != nil {
		return fmt.Errorf("unmarshal validator key: %w", err)
	}

	// Build the structure with eth_address at the top
	keyJSON := ValidatorKeyJSON{
		EthAddress: ethAddr,
		Address:    original.Address,
		PubKey:     original.PubKey,
		PrivKey:    original.PrivKey,
	}

	// Marshal and write
	data, err := json.MarshalIndent(keyJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal validator key: %w", err)
	}

	if err := os.WriteFile(privValKeyFile, data, 0600); err != nil {
		return fmt.Errorf("write validator key file: %w", err)
	}

	return nil
}

// InitNode initializes a new OpenAudio node at cfg.CometBFT.RootDir.
// It creates CometBFT keys, a default genesis, and writes merged configuration.
func InitNode(cfg *Config, preset string, ethKey string) error {
	rootDir := cfg.CometBFT.RootDir

	if err := createNodeDirectories(rootDir); err != nil {
		return err
	}

	privValKeyFile := cfg.CometBFT.PrivValidatorKeyFile()
	privValStateFile := cfg.CometBFT.PrivValidatorStateFile()
	nodeKeyFile := cfg.CometBFT.NodeKeyFile()

	pv, _, err := generateNodeKeys(privValKeyFile, privValStateFile, nodeKeyFile)
	if err != nil {
		return err
	}

	// Add eth_address to priv_validator_key.json
	if err := AddEthAddressToValidatorKey(privValKeyFile, pv); err != nil {
		return fmt.Errorf("add eth address to validator key: %w", err)
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
