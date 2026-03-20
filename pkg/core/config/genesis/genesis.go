package genesis

import (
	"embed"
	"fmt"

	"github.com/cometbft/cometbft/types"
)

//go:embed sandbox.json
var sandboxGenesis embed.FS

//go:embed dev.json
var devGenesis embed.FS

//go:embed stage.json
var stageGenesis embed.FS

//go:embed prod.json
var prodGenesis embed.FS

//go:embed prod-v2.json
var prodV2Genesis embed.FS

//go:embed dev-v2.json
var devV2Genesis embed.FS

func Read(environment string) (*types.GenesisDoc, error) {
	switch environment {
	case "dev-v2":
		data, err := devV2Genesis.ReadFile("dev-v2.json")
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded file: %v", err)
		}
		genDoc, err := types.GenesisDocFromJSON(data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal dev-v2.json into genesis: %v", err)
		}
		return genDoc, nil
	case "prod-v2", "mainnet-v2":
		data, err := prodV2Genesis.ReadFile("prod-v2.json")
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded file: %v", err)
		}
		genDoc, err := types.GenesisDocFromJSON(data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal prod-v2.json into genesis: %v", err)
		}
		return genDoc, nil
	case "prod", "production", "mainnet":
		data, err := prodGenesis.ReadFile("prod.json")
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded file: %v", err)
		}
		genDoc, err := types.GenesisDocFromJSON(data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal prod.json into genesis: %v", err)
		}
		return genDoc, nil
	case "stage", "staging", "testnet":
		data, err := stageGenesis.ReadFile("stage.json")
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded file: %v", err)
		}
		genDoc, err := types.GenesisDocFromJSON(data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal stage.json into genesis: %v", err)
		}
		return genDoc, nil
	case "sandbox":
		data, err := sandboxGenesis.ReadFile("sandbox.json")
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded file: %v", err)
		}
		genDoc, err := types.GenesisDocFromJSON(data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal stage.json into genesis: %v", err)
		}
		return genDoc, nil
	default:
		data, err := devGenesis.ReadFile("dev.json")
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded file: %v", err)
		}
		genDoc, err := types.GenesisDocFromJSON(data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal dev.json into genesis: %v", err)
		}
		return genDoc, nil
	}
}
