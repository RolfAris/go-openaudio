package config

import (
	"os"
	"path/filepath"

	cmcfg "github.com/cometbft/cometbft/config"
)

const (
	DefaultOpenAudioConfigDir = ".openaudio"

	// populated by ldflags
	Tag    = ""
	GitSHA = ""
)

// Root configuration for OpenAudio.
type Config struct {
	CometBFT  *cmcfg.Config    `mapstructure:",squash"`
	OpenAudio *OpenAudioConfig `mapstructure:"openaudio"`
}

func DefaultConfig() *Config {
	home := os.ExpandEnv(filepath.Join("$HOME", DefaultOpenAudioConfigDir))
	cmt := cmcfg.DefaultConfig()
	cmt.RootDir = home

	return &Config{
		CometBFT:  cmt,
		OpenAudio: DefaultOpenAudioConfig(home),
	}
}

type OpenAudioConfig struct {
	Version  VersionConfig  `mapstructure:"version"`
	Eth      EthConfig      `mapstructure:"eth"`
	DB       DBConfig       `mapstructure:"db"`
	Blob     BlobConfig     `mapstructure:"blob"`
	Operator OperatorConfig `mapstructure:"operator"`
	Server   ServerConfig   `mapstructure:"server"`
}

func DefaultOpenAudioConfig(home string) *OpenAudioConfig {
	return &OpenAudioConfig{
		Version:  DefaultVersionConfig(),
		Eth:      DefaultEthConfig(),
		DB:       DefaultDBConfig(),
		Blob:     DefaultBlobConfig(home),
		Operator: DefaultOperatorConfig(),
		Server:   DefaultServerConfig(home),
	}
}

type VersionConfig struct {
	Tag    string `mapstructure:"tag"`
	GitSHA string `mapstructure:"git_sha"`
}

func DefaultVersionConfig() VersionConfig {
	return VersionConfig{Tag: Tag, GitSHA: GitSHA}
}

type EthConfig struct {
	RpcURL          string `mapstructure:"rpcurl"`
	RegistryAddress string `mapstructure:"registryaddress"`
}

func DefaultEthConfig() EthConfig {
	return EthConfig{
		RpcURL:          "https://mainnet.infura.io/v3/YOUR_KEY",
		RegistryAddress: "",
	}
}

type DBConfig struct {
	PostgresDSN string `mapstructure:"postgres_dsn"`
}

func DefaultDBConfig() DBConfig {
	return DBConfig{
		PostgresDSN: "postgres://postgres:postgres@localhost:5432/openaudio?sslmode=disable",
	}
}

type BlobConfig struct {
	BlobStoreDSN         string `mapstructure:"blob_store_dsn"`
	MoveFromBlobStoreDSN string `mapstructure:"move_from_blob_store_dsn"`
}

func DefaultBlobConfig(home string) BlobConfig {
	blobDir := filepath.Join(home, "data", "blobs")
	return BlobConfig{
		BlobStoreDSN:         "file://" + blobDir,
		MoveFromBlobStoreDSN: "",
	}
}

type OperatorConfig struct {
	Address  string `mapstructure:"address"`
	PrivKey  string `mapstructure:"privkey"`
	Endpoint string `mapstructure:"endpoint"`
}

func DefaultOperatorConfig() OperatorConfig {
	return OperatorConfig{
		Endpoint: "http://localhost:44000",
	}
}

type ServerConfig struct {
	Port      string        `mapstructure:"port"`
	HTTPSPort string        `mapstructure:"https_port"`
	Hostname  string        `mapstructure:"hostname"`
	H2C       bool          `mapstructure:"h2c"`
	TLS       TLSConfig     `mapstructure:"tls"`
	Console   ConsoleConfig `mapstructure:"console"`
	Socket    SocketConfig  `mapstructure:"socket"`
}

func DefaultServerConfig(home string) ServerConfig {
	return ServerConfig{
		Port:      "8080",
		HTTPSPort: "8443",
		Hostname:  "127.0.0.1",
		H2C:       true,
		TLS:       DefaultTLSConfig(home),
		Console:   DefaultConsoleConfig(),
		Socket:    DefaultSocketConfig(home),
	}
}

type TLSConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	SelfSigned bool   `mapstructure:"self_signed"`
	CertDir    string `mapstructure:"cert_dir"`
	CacheDir   string `mapstructure:"cache_dir"`
}

func DefaultTLSConfig(home string) TLSConfig {
	return TLSConfig{
		Enabled:    false,
		SelfSigned: false,
		CertDir:    filepath.Join(home, "config", "certs"),
		CacheDir:   filepath.Join(home, "config", "cache"),
	}
}

type ConsoleConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	SubRoute string `mapstructure:"subroute"`
}

func DefaultConsoleConfig() ConsoleConfig {
	return ConsoleConfig{
		Enabled:  true,
		SubRoute: "/console",
	}
}

type SocketConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

func DefaultSocketConfig(home string) SocketConfig {
	return SocketConfig{
		Enabled: true,
		Path:    filepath.Join(home, "openaudio.sock"),
	}
}

type StorageConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

func DefaultStorageConfig() StorageConfig {
	return StorageConfig{Enabled: true}
}

type CoreConfig struct {
	OnlyCore bool   `mapstructure:"only_core"`
	RootDir  string `mapstructure:"root_dir"`
}

func DefaultCoreConfig(home string) CoreConfig {
	return CoreConfig{OnlyCore: false, RootDir: home}
}

type UptimeConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

func DefaultUptimeConfig() UptimeConfig {
	return UptimeConfig{Enabled: true}
}
