package config

// json structure that is passed to genesis doc
// all nodes start with the same config
// values in here must be changed through governance
type GenesisData struct {
	Pow POWGenesisConfig `json:"pow"`
	Pos POSGenesisConfig `json:"pos"`
}

type POWGenesisConfig struct {
	BlockInterval uint32 `json:"block_interval"`
	JailThreshold uint32 `json:"jail_threshold"`
}

type POSGenesisConfig struct {
	BlockHashMatcher        string `json:"block_hash_matcher"`
	ChallengeTimeoutSeconds uint32 `json:"challenge_timeout_seconds"`
}

type StorageGenesisConfig struct {
	ReplicationFactor uint16 `json:"replication_factor"`
}

type OracleGenesisConfig struct {
	TakeDownNotifiers []string `json:"takedown_oracles"`
}
