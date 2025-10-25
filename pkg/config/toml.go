package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/OpenAudio/go-openaudio/pkg/common"
	cmcfg "github.com/cometbft/cometbft/config"
)

//go:embed openaudio.toml.tpl
var openaudioConfigTemplate string

// WriteMergedConfigFile writes the combined CometBFT + OpenAudio config.toml
func WriteMergedConfigFile(cfg *Config, home string) error {
	rootDir := home
	configDir := filepath.Join(rootDir, "config")
	configPath := filepath.Join(configDir, "config.toml")

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if common.FileExists(configPath) {
		return nil
	}

	// 1. Write CometBFT defaults using their embedded template
	cmcfg.WriteConfigFile(configPath, cfg.CometBFT)

	// 2. Render OpenAudio-specific sections
	tmpl, err := template.New("openaudio").Parse(openaudioConfigTemplate)
	if err != nil {
		return fmt.Errorf("parse openaudio template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return fmt.Errorf("execute openaudio template: %w", err)
	}

	// 3. Append rendered OpenAudio config to file
	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open config file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString("\n\n" + buf.String()); err != nil {
		return fmt.Errorf("append openaudio config: %w", err)
	}

	return nil
}
