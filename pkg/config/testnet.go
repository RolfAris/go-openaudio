package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/libs/bytes"
	"github.com/cometbft/cometbft/types"
	cmttime "github.com/cometbft/cometbft/types/time"
	"github.com/google/uuid"
)

// NodeConfig holds configuration for a single node in the testnet.
type NodeConfig struct {
	Preset       string
	Index        int
	HomeDir      string
	Moniker      string
	ValidatorKey crypto.PubKey
	NodeID       string
	P2PAddress   string
}

// GenerateTestnet creates a multi-node testnet with proper docker networking.
func GenerateTestnet(rootDir, chainID string, counts map[string]int) error {
	// Create root directory
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return fmt.Errorf("create root dir: %w", err)
	}

	// Generate chain ID if not provided
	if chainID == "" {
		hash := sha256.Sum256([]byte(uuid.NewString()))
		chainID = fmt.Sprintf("openaudio-testnet-%x", hash[:6])
	}

	// Phase 1: Generate all node keys and collect validator info
	var allNodes []NodeConfig
	var validatorNodes []NodeConfig

	presets := []string{"validator", "seed", "archive", "rpc", "light"}
	for _, preset := range presets {
		count := counts[preset]
		for i := 1; i <= count; i++ {
			moniker := fmt.Sprintf("%s-%d", preset, i)
			homeDir := filepath.Join(rootDir, moniker)

			// Create directories
			if err := createNodeDirectories(homeDir); err != nil {
				return fmt.Errorf("create directories for %s: %w", moniker, err)
			}

			// Generate keys
			privValKeyFile := filepath.Join(homeDir, "config", "priv_validator_key.json")
			privValStateFile := filepath.Join(homeDir, "data", "priv_validator_state.json")
			nodeKeyFile := filepath.Join(homeDir, "config", "node_key.json")

			pv, nodeKey, err := generateNodeKeys(privValKeyFile, privValStateFile, nodeKeyFile)
			if err != nil {
				return fmt.Errorf("generate keys for %s: %w", moniker, err)
			}

			pubKey, err := pv.GetPubKey()
			if err != nil {
				return fmt.Errorf("get pubkey for %s: %w", moniker, err)
			}

			nodeConfig := NodeConfig{
				Preset:       preset,
				Index:        i,
				HomeDir:      homeDir,
				Moniker:      moniker,
				ValidatorKey: pubKey,
				NodeID:       string(nodeKey.ID()),
				P2PAddress:   fmt.Sprintf("%s:26656", moniker),
			}

			allNodes = append(allNodes, nodeConfig)

			// Only validators go in the genesis
			if preset == "validator" {
				validatorNodes = append(validatorNodes, nodeConfig)
			}
		}
	}

	// Phase 2: Create shared genesis document
	genDoc := types.GenesisDoc{
		ChainID:         chainID,
		GenesisTime:     cmttime.Now(),
		ConsensusParams: types.DefaultConsensusParams(),
	}

	genDoc.ConsensusParams.Validator.PubKeyTypes = []string{"secp256k1"}

	// Add validators to genesis
	for _, node := range validatorNodes {
		genDoc.Validators = append(genDoc.Validators, types.GenesisValidator{
			Address: node.ValidatorKey.Address(),
			PubKey:  node.ValidatorKey,
			Power:   10,
			Name:    node.Moniker,
		})
	}

	// Set app state
	appState, _ := json.Marshal(DefaultGenesisData())
	genDoc.AppState = appState
	appHashSha := sha256.Sum256(appState)
	appHash := bytes.HexBytes(appHashSha[:])
	genDoc.AppHash = appHash

	// Phase 3: Write configs for all nodes
	for _, node := range allNodes {
		// Create config
		cfg := DefaultConfig()
		cfg.SetHome(node.HomeDir)
		cfg.CometBFT.Moniker = node.Moniker

		// Configure for docker networking
		configureDockerNetworking(cfg, node, validatorNodes)

		// Write genesis file
		genFile := filepath.Join(node.HomeDir, "config", "genesis.json")
		if err := genDoc.SaveAs(genFile); err != nil {
			return fmt.Errorf("save genesis for %s: %w", node.Moniker, err)
		}

		// Write merged config file
		if err := WriteMergedConfigFile(cfg, node.HomeDir); err != nil {
			return fmt.Errorf("write config for %s: %w", node.Moniker, err)
		}
	}

	fmt.Printf("Successfully generated testnet with %d nodes in %s\n", len(allNodes), rootDir)
	fmt.Printf("Chain ID: %s\n", chainID)
	fmt.Printf("Validators: %d\n", len(validatorNodes))

	return nil
}

// configureDockerNetworking sets up CometBFT config for docker networking.
func configureDockerNetworking(cfg *Config, node NodeConfig, validators []NodeConfig) {
	// P2P configuration for docker
	cfg.CometBFT.P2P.ListenAddress = "tcp://0.0.0.0:26656"
	cfg.CometBFT.P2P.ExternalAddress = fmt.Sprintf("tcp://%s:26656", node.Moniker)

	// RPC configuration for docker
	cfg.CometBFT.RPC.ListenAddress = "tcp://0.0.0.0:26657"

	// Configure persistent peers - all nodes connect to validators
	var peers []string
	for _, val := range validators {
		// Don't connect to self
		if val.Moniker != node.Moniker {
			peer := fmt.Sprintf("%s@%s:26656", val.NodeID, val.Moniker)
			peers = append(peers, peer)
		}
	}
	cfg.CometBFT.P2P.PersistentPeers = strings.Join(peers, ",")

	// Allow duplicate IPs for docker networking
	cfg.CometBFT.P2P.AllowDuplicateIP = true
}

