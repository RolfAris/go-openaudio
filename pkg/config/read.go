package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/OpenAudio/go-openaudio/pkg/common"
	cmprivval "github.com/cometbft/cometbft/privval"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
)

func Load(path string, home string) (*Config, error) {
	cfg := DefaultConfig()
	cfg.SetHome(home)

	// 1. Initialize viper
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")

	// read base file if it exists
	if err := v.ReadInConfig(); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	// 2. Bind environment variables
	v.SetEnvPrefix("OPENAUDIO")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 3. Overlay CLI flags (--key=value)
	// Cobra will bind its flags into viper before Load() runs
	// (see StartCmd below)

	// 4. Unmarshal
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.CometBFT.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("invalid cometbft config: %w", err)
	}

	keyPath := cfg.CometBFT.PrivValidatorKeyFile()
	keyStatePath := cfg.CometBFT.PrivValidatorStateFile()

	if _, err := os.Stat(keyPath); err == nil {
		pvKey := cmprivval.LoadFilePV(keyPath, keyStatePath)
		privKey := pvKey.Key.PrivKey

		// Parse the secp256k1 key into go-ethereum format
		ethKey, err := crypto.ToECDSA(privKey.Bytes())
		if err != nil {
			return nil, fmt.Errorf("invalid secp256k1 privkey: %w", err)
		}

		addr := common.PrivKeyToAddress(ethKey)
		privHex := hex.EncodeToString(crypto.FromECDSA(ethKey))

		cfg.OpenAudio.Operator.Address = addr
		cfg.OpenAudio.Operator.PrivKey = privHex
	}

	return cfg, nil
}
