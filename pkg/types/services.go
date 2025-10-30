package types

import (
	"context"

	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
)

type CoreService interface {
	GetBlock(ctx context.Context) (*v1.Block, error)
}

type StorageService interface {
	GetUpload(ctx context.Context) (string, error)
}

type ProofOfStorageService interface {
	SendPoSChallenge(ctx context.Context, blockHash []byte, blockHeight int64) error
}

type ProofOfWorkService interface {
	ShouldProposeNewRollup(ctx context.Context, height int64) bool
}

type RegistryBridgeService interface {
}
