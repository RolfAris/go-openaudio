package config

import (
	"path/filepath"
	"time"

	cmcfg "github.com/cometbft/cometbft/config"
	"go.uber.org/zap"
)

const (
	// Version info (populated by ldflags)
	Tag    = ""
	GitSHA = ""

	// Directory layout
	DefaultHomeDir   = ".openaudio"
	DefaultConfigDir = "config"
	DefaultDataDir   = "data"
	DefaultBlobsDir  = "blobs"
	DefaultCertsDir  = "certs"
	DefaultCacheDir  = "cache"

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

	// Defaults for echo server
	DefaultRequestTimeout = 5 * time.Second
	DefaultIPRateLimit    = 100
)

// Root configuration for OpenAudio.
type Config struct {
	CometBFT  *cmcfg.Config    `mapstructure:",squash"`
	OpenAudio *OpenAudioConfig `mapstructure:"openaudio"`
}

func DefaultConfig() *Config {
	cmt := cmcfg.DefaultConfig()
	cmt.RootDir = DefaultHomeDir

	return &Config{
		CometBFT:  cmt,
		OpenAudio: DefaultOpenAudioConfig(),
	}
}

func (c *Config) SetHome(home string) {
	// update CometBFT root
	c.CometBFT.SetRoot(home)

	// update top-level home
	c.OpenAudio.Home = home

	// update sub-configs that have derived paths
	c.OpenAudio.Version.Home = home
	c.OpenAudio.Eth.Home = home
	c.OpenAudio.DB.Home = home
	c.OpenAudio.Blob.Home = home
	c.OpenAudio.Operator.Home = home
	c.OpenAudio.Server.Home = home

	// recompute derived directories
	c.OpenAudio.Blob.BlobStoreDSN = "file://" + filepath.Join(home, DefaultDataDir, DefaultBlobsDir)
	c.OpenAudio.Server.Socket.Path = filepath.Join(home, DefaultSocketFileName)
	c.OpenAudio.Server.TLS.CertDir = filepath.Join(home, DefaultConfigDir, DefaultCertsDir)
	c.OpenAudio.Server.TLS.CacheDir = filepath.Join(home, DefaultConfigDir, DefaultCacheDir)

	// propagate home for server nested configs that still hold it
	c.OpenAudio.Server.TLS.Home = home
	c.OpenAudio.Server.Console.Home = home
	c.OpenAudio.Server.Socket.Home = home
}

// OpenAudioConfig holds all OpenAudio-specific configuration.
type OpenAudioConfig struct {
	Home     string          `mapstructure:"home"`
	Logger   *zap.Config     `mapstructure:"logger"`
	Version  *VersionConfig  `mapstructure:"version"`
	Eth      *EthConfig      `mapstructure:"eth"`
	DB       *DBConfig       `mapstructure:"db"`
	Blob     *BlobConfig     `mapstructure:"blob"`
	Operator *OperatorConfig `mapstructure:"operator"`
	Server   *ServerConfig   `mapstructure:"server"`
}

func DefaultOpenAudioConfig() *OpenAudioConfig {
	loggerCfg := zap.NewProductionConfig()
	return &OpenAudioConfig{
		Home:     DefaultHomeDir,
		Logger:   &loggerCfg,
		Version:  DefaultVersionConfig(),
		Eth:      DefaultEthConfig(),
		DB:       DefaultDBConfig(),
		Blob:     DefaultBlobConfig(),
		Operator: DefaultOperatorConfig(),
		Server:   DefaultServerConfig(),
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

func DefaultVersionConfig() *VersionConfig {
	return &VersionConfig{Home: DefaultHomeDir, Tag: Tag, GitSHA: GitSHA}
}

type EthConfig struct {
	Home            string `mapstructure:"home"`
	RpcURL          string `mapstructure:"rpcurl"`
	RegistryAddress string `mapstructure:"registryaddress"`
}

func DefaultEthConfig() *EthConfig {
	return &EthConfig{
		Home:            DefaultHomeDir,
		RpcURL:          DefaultRPCURL,
		RegistryAddress: "",
	}
}

type DBConfig struct {
	Home        string `mapstructure:"home"`
	PostgresDSN string `mapstructure:"postgres_dsn"`
}

func DefaultDBConfig() *DBConfig {
	return &DBConfig{
		Home:        DefaultHomeDir,
		PostgresDSN: DefaultPostgresDSN,
	}
}

type BlobConfig struct {
	Home                 string `mapstructure:"home"`
	BlobStoreDSN         string `mapstructure:"blob_store_dsn"`
	MoveFromBlobStoreDSN string `mapstructure:"move_from_blob_store_dsn"`
}

func DefaultBlobConfig() *BlobConfig {
	blobDir := filepath.Join(DefaultHomeDir, DefaultDataDir, DefaultBlobsDir)
	return &BlobConfig{
		Home:                 DefaultHomeDir,
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

func DefaultOperatorConfig() *OperatorConfig {
	return &OperatorConfig{
		Home:     DefaultHomeDir,
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
	Echo      *EchoConfig    `mapstructure:"echo"`
}

func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Home:      DefaultHomeDir,
		Port:      DefaultHTTPPort,
		HTTPSPort: DefaultHTTPSPort,
		Hostname:  DefaultHostname,
		H2C:       true,
		TLS:       DefaultTLSConfig(),
		Console:   DefaultConsoleConfig(),
		Socket:    DefaultSocketConfig(),
		Echo:      DefaultEchoConfig(),
	}
}

type TLSConfig struct {
	Home       string `mapstructure:"home"`
	Enabled    bool   `mapstructure:"enabled"`
	SelfSigned bool   `mapstructure:"self_signed"`
	CertDir    string `mapstructure:"cert_dir"`
	CacheDir   string `mapstructure:"cache_dir"`
}

func DefaultTLSConfig() *TLSConfig {
	return &TLSConfig{
		Home:       DefaultHomeDir,
		Enabled:    false,
		SelfSigned: false,
		CertDir:    filepath.Join(DefaultHomeDir, DefaultConfigDir, DefaultCertsDir),
		CacheDir:   filepath.Join(DefaultHomeDir, DefaultConfigDir, DefaultCacheDir),
	}
}

type ConsoleConfig struct {
	Home     string `mapstructure:"home"`
	Enabled  bool   `mapstructure:"enabled"`
	SubRoute string `mapstructure:"subroute"`
}

func DefaultConsoleConfig() *ConsoleConfig {
	return &ConsoleConfig{
		Home:     DefaultHomeDir,
		Enabled:  true,
		SubRoute: "/console",
	}
}

type SocketConfig struct {
	Home    string `mapstructure:"home"`
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

func DefaultSocketConfig() *SocketConfig {
	return &SocketConfig{
		Home:    DefaultHomeDir,
		Enabled: true,
		Path:    filepath.Join(DefaultHomeDir, DefaultSocketFileName),
	}
}

type EchoConfig struct {
	IPRateLimit    float64       `mapstructure:"ip_rate_limit"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
}

func DefaultEchoConfig() *EchoConfig {
	return &EchoConfig{
		IPRateLimit:    DefaultIPRateLimit,
		RequestTimeout: DefaultRequestTimeout,
	}
}
