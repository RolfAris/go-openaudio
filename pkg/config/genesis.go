package config

// json structure that is passed to genesis doc
// all nodes start with the same config
// values in here must be changed through OpenAudio governance
type GenesisData struct {
	Auditor   AuditorGenesisConfig   `json:"auditor"`
	Storage   StorageGenesisConfig   `json:"storage"`
	Oracle    OracleGenesisConfig    `json:"oracle"`
	Eth       EthGenesisConfig       `json:"eth"`
	Validator ValidatorGenesisConfig `json:"validator"`
}

func DefaultGenesisData() GenesisData {
	return GenesisData{
		Auditor: DefaultAuditorGenesisConfig(),
		Storage: DefaultStorageGenesisConfig(),
		Oracle:  DefaultOracleGenesisConfig(),
		Eth:     DefaultEthGenesisConfig(),
	}
}

type AuditorGenesisConfig struct {
	SlaRollupInterval int `json:"sla_rollup_interval"` // interval to propose new SLA rollups in blocks
}

func DefaultAuditorGenesisConfig() AuditorGenesisConfig {
	return AuditorGenesisConfig{
		SlaRollupInterval: 100, // 100 blocks
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

type ValidatorGenesisConfig struct {
	/* Attestation Thresholds */
	AttRegistrationMin                   int `json:"att_registration_min"`                    // minimum number of attestations needed to register a new node
	AttRegistrationRSize                 int `json:"att_registration_r_size"`                 // rendezvous size for registration attestations (should be >= to AttRegistrationMin)
	AttDeregistrationMin                 int `json:"att_deregistration_min"`                  // minimum number of attestations needed to deregister a node
	AttDeregistrationRSize               int `json:"att_deregistration_r_size"`               // rendezvous size for deregistration attestations (should be >= to AttDeregistrationMin)
	MaxRegistrationAttestationValidity   int `json:"max_registration_attestation_validity"`   // maximum time in seconds that a registration attestation is valid
	MaxDeregistrationAttestationValidity int `json:"max_deregistration_attestation_validity"` // maximum time in seconds that a deregistration attestation is valid
	ValidatorVotingPower                 int `json:"validator_voting_power"`                  // voting power of the validator
}

func DefaultValidatorGenesisConfig() ValidatorGenesisConfig {
	return ValidatorGenesisConfig{
		AttRegistrationMin:                   5,
		AttRegistrationRSize:                 10,
		AttDeregistrationMin:                 5,
		AttDeregistrationRSize:               10,
		MaxRegistrationAttestationValidity:   24 * 60 * 60, // 24 hours
		MaxDeregistrationAttestationValidity: 24 * 60 * 60, // 24 hours
		ValidatorVotingPower:                 10,
	}
}
