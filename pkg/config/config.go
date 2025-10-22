package config

import (
	cmcfg "github.com/cometbft/cometbft/config"
)

// Root configuration for OpenAudio.
type Config struct {
	CometBFT  *cmcfg.Config    `mapstructure:",squash"`
	OpenAudio *OpenAudioConfig `mapstructure:"openaudio"`
}

type OpenAudioConfig struct {
	Version  VersionConfig  `mapstructure:"version"`
	Eth      EthConfig      `mapstructure:"eth"`
	DB       DBConfig       `mapstructure:"db"`
	Blob     BlobConfig     `mapstructure:"blob"`
	Operator OperatorConfig `mapstructure:"operator"`
	Server   ServerConfig   `mapstructure:"server"`
}

type VersionConfig struct {
	Tag    string `mapstructure:"tag"`
	GitSHA string `mapstructure:"git_sha"`
}

type EthConfig struct {
	RpcURL          string `mapstructure:"rpcurl"`
	RegistryAddress string `mapstructure:"registryaddress"`
}

type DBConfig struct {
	PostgresDSN string `mapstructure:"postgres_dsn"`
}

type BlobConfig struct {
	BlobStoreDSN         string `mapstructure:"blob_store_dsn"`
	MoveFromBlobStoreDSN string `mapstructure:"move_from_blob_store_dsn"`
}

type OperatorConfig struct {
	PrivKey  string `mapstructure:"privkey"`
	Endpoint string `mapstructure:"endpoint"`
}

type ServerConfig struct {
	Port      string        `mapstructure:"port"`
	HTTPSPort string        `mapstructure:"https_port"`
	Hostname  string        `mapstructure:"hostname"`
	H2C       bool          `mapstructure:"h2c"` // Enable HTTP/2 cleartext for gRPC
	TLS       TLSConfig     `mapstructure:"tls"`
	Console   ConsoleConfig `mapstructure:"console"`
	Socket    SocketConfig  `mapstructure:"socket"`
}

type TLSConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	SelfSigned bool   `mapstructure:"self_signed"`
	CertDir    string `mapstructure:"cert_dir"`
	CacheDir   string `mapstructure:"cache_dir"`
}

type ConsoleConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	SubRoute string `mapstructure:"subroute"`
}

type SocketConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

type StorageConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

type CoreConfig struct {
	OnlyCore bool   `mapstructure:"only_core"`
	RootDir  string `mapstructure:"root_dir"`
}

type UptimeConfig struct {
	Enabled bool `mapstructure:"enabled"`
}
