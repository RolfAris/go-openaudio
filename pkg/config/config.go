package config

import (
	"fmt"
	"os"
	"strings"

	cmcfg "github.com/cometbft/cometbft/config"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
)

// Root configuration for OpenAudio.
type Config struct {
	CometBFT  *cmcfg.Config    `mapstructure:",squash"`
	OpenAudio *OpenAudioConfig `mapstructure:"openaudio"`
}

type OpenAudioConfig struct {
	Eth      EthConfig      `mapstructure:"eth"`
	DB       DBConfig       `mapstructure:"db"`
	Operator OperatorConfig `mapstructure:"operator"`
	Server   ServerConfig   `mapstructure:"server"`
}

type EthConfig struct {
	RpcURL          string `mapstructure:"rpcurl"`
	RegistryAddress string `mapstructure:"registryaddress"`
}

type DBConfig struct {
	PGConn string `mapstructure:"pgconn"`
}

type OperatorConfig struct {
	PrivKey  string `mapstructure:"privkey"`
	Endpoint string `mapstructure:"endpoint"`
}

type ServerConfig struct {
	ConnectRPC ConnectRPCConfig `mapstructure:"connectrpc"`
	GRPC       GRPCConfig       `mapstructure:"grpc"`
	Console    ConsoleConfig    `mapstructure:"console"`
}

type ConnectRPCConfig struct {
	HttpsPort uint `mapstructure:"httpsport"`
	HttpPort  uint `mapstructure:"httpport"`
}

type GRPCConfig struct {
	Port uint `mapstructure:"port"`
}

type ConsoleConfig struct {
	Serve    bool   `mapstructure:"serve"`
	SubRoute string `mapstructure:"subroute"`
}

// Load merges config.toml → env vars → CLI (--key=value) in that order.
func Load(path string, prefix string) (*Config, error) {
	k := koanf.New(".")

	// 1. TOML base
	if err := k.Load(file.Provider(path), toml.Parser()); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load toml: %w", err)
	}

	// 2. Env overrides (e.g. OPENAUDIO_DB_PGCONN)
	if err := k.Load(env.Provider(prefix, ".", func(s string) string {
		return strings.ToLower(strings.TrimPrefix(strings.ReplaceAll(s, "_", "."), strings.ToLower(prefix+".")))
	}), nil); err != nil {
		return nil, fmt.Errorf("load env: %w", err)
	}

	// 3. CLI --key=value overrides
	overrides := map[string]interface{}{}
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "--") && strings.Contains(a, "=") {
			kv := strings.SplitN(strings.TrimPrefix(a, "--"), "=", 2)
			overrides[strings.ToLower(kv[0])] = kv[1]
		}
	}
	if len(overrides) > 0 {
		if err := k.Load(confmap.Provider(overrides, "."), nil); err != nil {
			return nil, fmt.Errorf("load cli: %w", err)
		}
	}

	// 4. Unmarshal into struct
	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// 5. Apply CometBFT defaults & validation
	if cfg.CometBFT == nil {
		cfg.CometBFT = cmcfg.DefaultConfig()
	}
	if err := cfg.CometBFT.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("invalid cometbft config: %w", err)
	}

	return &cfg, nil
}
