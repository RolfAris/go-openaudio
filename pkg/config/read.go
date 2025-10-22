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
