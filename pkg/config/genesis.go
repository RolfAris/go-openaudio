package config

// json structure that is passed to genesis doc
// all nodes start with the same config
// values in here must be changed through OpenAudio governance
type GenesisData struct {
	Pow     POWGenesisConfig     `json:"pow"`
	Pos     POSGenesisConfig     `json:"pos"`
	Storage StorageGenesisConfig `json:"storage"`
	Oracle  OracleGenesisConfig  `json:"oracle"`
	Eth     EthGenesisConfig     `json:"eth"`
}

func DefaultGenesisData() GenesisData {
	return GenesisData{
		Pow:     DefaultPOWGenesisConfig(),
		Pos:     DefaultPOSGenesisConfig(),
		Storage: DefaultStorageGenesisConfig(),
		Oracle:  DefaultOracleGenesisConfig(),
		Eth:     DefaultEthGenesisConfig(),
	}
}

type POWGenesisConfig struct {
	BlockInterval uint32 `json:"block_interval"`
	JailThreshold uint32 `json:"jail_threshold"`
}

func DefaultPOWGenesisConfig() POWGenesisConfig {
	return POWGenesisConfig{
		BlockInterval: 6,   // 6 seconds per block target
		JailThreshold: 100, // 100 missed blocks = jailed
	}
}

type POSGenesisConfig struct {
	BlockHashMatcher        string `json:"block_hash_matcher"`
	ChallengeTimeoutSeconds uint32 `json:"challenge_timeout_seconds"`
}

func DefaultPOSGenesisConfig() POSGenesisConfig {
	return POSGenesisConfig{
		BlockHashMatcher:        "0x0000", // 4-bit prefix target
		ChallengeTimeoutSeconds: 60,       // 1 minute response window
	}
}

type StorageGenesisConfig struct {
	ReplicationFactor uint16 `json:"replication_factor"`
}

func DefaultStorageGenesisConfig() StorageGenesisConfig {
	return StorageGenesisConfig{
		ReplicationFactor: 3, // store blobs across 3 nodes by default
	}
}

type OracleGenesisConfig struct {
	TakeDownNotifiers []string `json:"takedown_oracles"`
}

func DefaultOracleGenesisConfig() OracleGenesisConfig {
	return OracleGenesisConfig{}
}

type EthGenesisConfig struct {
	RegistryAddress string `json:"registry_address"`
}

func DefaultEthGenesisConfig() EthGenesisConfig {
	return EthGenesisConfig{
		RegistryAddress: "0x0000000000000000000000000000000000000000", // placeholder
	}
}
