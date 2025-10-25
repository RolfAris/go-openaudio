package config

import (
	"path/filepath"

	cmcfg "github.com/cometbft/cometbft/config"
)

const (
	// Version info (populated by ldflags)
	Tag    = ""
	GitSHA = ""

	// Directory layout
	DefaultOpenAudioDir = ".openaudio"
	DefaultConfigDir    = "config"
	DefaultDataDir      = "data"
	DefaultBlobsDir     = "blobs"
	DefaultCertsDir     = "certs"
	DefaultCacheDir     = "cache"

	// File names
	DefaultConfigFileName  = "config.toml"
	DefaultGenesisFileName = "genesis.json"
	DefaultSocketFileName  = "openaudio.sock"

	// Logging
	LogFormatPlain  = "plain"
	LogFormatJSON   = "json"
	DefaultLogLevel = "info"

	// Defaults for networking and runtime
	DefaultHTTPPort  = "8080"
	DefaultHTTPSPort = "8443"
	DefaultHostname  = "127.0.0.1"
	DefaultEndpoint  = "http://localhost:44000"
	DefaultRPCURL    = "http://localhost:8545"

	// Default DSNs
	DefaultPostgresDSN = "postgres://postgres:postgres@localhost:5432/openaudio?sslmode=disable"
)

// Root configuration for OpenAudio.
type Config struct {
	CometBFT  *cmcfg.Config    `mapstructure:",squash"`
	OpenAudio *OpenAudioConfig `mapstructure:"openaudio"`
}

func DefaultConfig() *Config {
	cmt := cmcfg.DefaultConfig()
	cmt.RootDir = DefaultOpenAudioDir

	return &Config{
		CometBFT:  cmt,
		OpenAudio: DefaultOpenAudioConfig(DefaultOpenAudioDir),
	}
}

func (c *Config) SetHome(home string) {
	c.CometBFT.SetRoot(home)

	c.OpenAudio.Home = home
	c.OpenAudio.Version.Home = home
	c.OpenAudio.Eth.Home = home
	c.OpenAudio.DB.Home = home
	c.OpenAudio.Blob.Home = home
	c.OpenAudio.Operator.Home = home
	c.OpenAudio.Server.Home = home

	c.OpenAudio.Server.TLS.Home = home
	c.OpenAudio.Server.Console.Home = home
	c.OpenAudio.Server.Socket.Home = home
}

// OpenAudioConfig holds all OpenAudio-specific configuration.
type OpenAudioConfig struct {
	Home     string          `mapstructure:"home"`
	Version  *VersionConfig  `mapstructure:"version"`
	Eth      *EthConfig      `mapstructure:"eth"`
	DB       *DBConfig       `mapstructure:"db"`
	Blob     *BlobConfig     `mapstructure:"blob"`
	Operator *OperatorConfig `mapstructure:"operator"`
	Server   *ServerConfig   `mapstructure:"server"`
}

func DefaultOpenAudioConfig(home string) *OpenAudioConfig {
	return &OpenAudioConfig{
		Home:     home,
		Version:  DefaultVersionConfig(home),
		Eth:      DefaultEthConfig(home),
		DB:       DefaultDBConfig(home),
		Blob:     DefaultBlobConfig(home),
		Operator: DefaultOperatorConfig(home),
		Server:   DefaultServerConfig(home),
	}
}

// ---------------------------------------------------------
// Sub-Configs
// ---------------------------------------------------------

type VersionConfig struct {
	Home   string `mapstructure:"home"`
	Tag    string `mapstructure:"tag"`
	GitSHA string `mapstructure:"git_sha"`
}

func DefaultVersionConfig(home string) *VersionConfig {
	return &VersionConfig{Home: home, Tag: Tag, GitSHA: GitSHA}
}

type EthConfig struct {
	Home            string `mapstructure:"home"`
	RpcURL          string `mapstructure:"rpcurl"`
	RegistryAddress string `mapstructure:"registryaddress"`
}

func DefaultEthConfig(home string) *EthConfig {
	return &EthConfig{
		Home:            home,
		RpcURL:          DefaultRPCURL,
		RegistryAddress: "",
	}
}

type DBConfig struct {
	Home        string `mapstructure:"home"`
	PostgresDSN string `mapstructure:"postgres_dsn"`
}

func DefaultDBConfig(home string) *DBConfig {
	return &DBConfig{
		Home:        home,
		PostgresDSN: DefaultPostgresDSN,
	}
}

type BlobConfig struct {
	Home                 string `mapstructure:"home"`
	BlobStoreDSN         string `mapstructure:"blob_store_dsn"`
	MoveFromBlobStoreDSN string `mapstructure:"move_from_blob_store_dsn"`
}

func DefaultBlobConfig(home string) *BlobConfig {
	blobDir := filepath.Join(home, DefaultDataDir, DefaultBlobsDir)
	return &BlobConfig{
		Home:                 home,
		BlobStoreDSN:         "file://" + blobDir,
		MoveFromBlobStoreDSN: "",
	}
}

type OperatorConfig struct {
	Home     string `mapstructure:"home"`
	Address  string `mapstructure:"address"`
	PrivKey  string `mapstructure:"privkey"`
	Endpoint string `mapstructure:"endpoint"`
}

func DefaultOperatorConfig(home string) *OperatorConfig {
	return &OperatorConfig{
		Home:     home,
		Endpoint: DefaultEndpoint,
	}
}

type ServerConfig struct {
	Home      string         `mapstructure:"home"`
	Port      string         `mapstructure:"port"`
	HTTPSPort string         `mapstructure:"https_port"`
	Hostname  string         `mapstructure:"hostname"`
	H2C       bool           `mapstructure:"h2c"`
	TLS       *TLSConfig     `mapstructure:"tls"`
	Console   *ConsoleConfig `mapstructure:"console"`
	Socket    *SocketConfig  `mapstructure:"socket"`
}

func DefaultServerConfig(home string) *ServerConfig {
	return &ServerConfig{
		Home:      home,
		Port:      DefaultHTTPPort,
		HTTPSPort: DefaultHTTPSPort,
		Hostname:  DefaultHostname,
		H2C:       true,
		TLS:       DefaultTLSConfig(home),
		Console:   DefaultConsoleConfig(home),
		Socket:    DefaultSocketConfig(home),
	}
}

type TLSConfig struct {
	Home       string `mapstructure:"home"`
	Enabled    bool   `mapstructure:"enabled"`
	SelfSigned bool   `mapstructure:"self_signed"`
	CertDir    string `mapstructure:"cert_dir"`
	CacheDir   string `mapstructure:"cache_dir"`
}

func DefaultTLSConfig(home string) *TLSConfig {
	return &TLSConfig{
		Home:       home,
		Enabled:    false,
		SelfSigned: false,
		CertDir:    filepath.Join(home, DefaultConfigDir, DefaultCertsDir),
		CacheDir:   filepath.Join(home, DefaultConfigDir, DefaultCacheDir),
	}
}

type ConsoleConfig struct {
	Home     string `mapstructure:"home"`
	Enabled  bool   `mapstructure:"enabled"`
	SubRoute string `mapstructure:"subroute"`
}

func DefaultConsoleConfig(home string) *ConsoleConfig {
	return &ConsoleConfig{
		Home:     home,
		Enabled:  true,
		SubRoute: "/console",
	}
}

type SocketConfig struct {
	Home    string `mapstructure:"home"`
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

func DefaultSocketConfig(home string) *SocketConfig {
	return &SocketConfig{
		Home:    home,
		Enabled: true,
		Path:    filepath.Join(home, DefaultSocketFileName),
	}
}
