package config

// json structure that is passed to genesis doc
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
