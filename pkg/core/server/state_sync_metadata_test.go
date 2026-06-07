package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/OpenAudio/go-openaudio/pkg/core/config"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	abciapi "github.com/cometbft/cometbft/api/cometbft/abci/v1"
	cometbfttypes "github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSnapshotMetadataDeclaresFullCoreHistory(t *testing.T) {
	s := &Server{
		config: &config.Config{
			ProposerAddress: "validator-a",
			GenesisFile:     &cometbfttypes.GenesisDoc{ChainID: "audius-test"},
		},
	}

	meta := s.snapshotMetadata()
	require.Equal(t, "validator-a", meta.Sender)
	require.Equal(t, "audius-test", meta.ChainID)
	require.NotNil(t, meta.CoreHistory)
	require.Equal(t, coreHistoryModeFullHistory, meta.CoreHistory.Mode)
	require.NoError(t, meta.validate("audius-test"))

	payload, err := json.Marshal(meta)
	require.NoError(t, err)

	var roundTripped Metadata
	require.NoError(t, json.Unmarshal(payload, &roundTripped))
	require.NoError(t, roundTripped.validate("audius-test"))
	require.Equal(t, coreHistoryModeFullHistory, roundTripped.CoreHistory.Mode)
}

func TestMetadataValidateAcceptsLegacySnapshotMetadata(t *testing.T) {
	meta := Metadata{
		Sender:  "validator-a",
		ChainID: "audius-test",
	}

	require.NoError(t, meta.validate("audius-test"))
}

func TestMetadataValidateRejectsChainMismatch(t *testing.T) {
	meta := Metadata{
		Sender:  "validator-a",
		ChainID: "wrong-chain",
	}

	require.ErrorContains(t, meta.validate("audius-test"), "chain ID mismatch")
}

func TestMetadataValidateCoreHistoryModes(t *testing.T) {
	tests := []struct {
		name        string
		coreHistory *CoreHistoryMetadata
		wantErr     string
	}{
		{
			name: "full history",
			coreHistory: &CoreHistoryMetadata{
				Mode: coreHistoryModeFullHistory,
			},
		},
		{
			name: "unknown future mode",
			coreHistory: &CoreHistoryMetadata{
				Mode: "partitioned_epoch_archive",
			},
			wantErr: "unknown core history snapshot mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := Metadata{
				Sender:      "validator-a",
				ChainID:     "audius-test",
				CoreHistory: tt.coreHistory,
			}

			err := meta.validate("audius-test")
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestApplySnapshotChunkAcceptsLegacySnapshotMetadata(t *testing.T) {
	s := &Server{
		config: &config.Config{
			RootDir:     t.TempDir(),
			GenesisFile: &cometbfttypes.GenesisDoc{ChainID: "audius-test"},
		},
		logger: zap.NewNop(),
	}

	metadata, err := json.Marshal(Metadata{
		Sender:  "validator-a",
		ChainID: "audius-test",
	})
	require.NoError(t, err)

	require.NoError(t, s.StoreOfferedSnapshot(&abciapi.Snapshot{
		Height:   123,
		Format:   1,
		Chunks:   2,
		Hash:     []byte("snapshot-hash"),
		Metadata: metadata,
	}))

	res, err := s.ApplySnapshotChunk(context.Background(), &abcitypes.ApplySnapshotChunkRequest{
		Index:  0,
		Sender: "validator-a",
		Chunk:  []byte("chunk-data"),
	})
	require.NoError(t, err)
	require.Equal(t, abcitypes.APPLY_SNAPSHOT_CHUNK_RESULT_ACCEPT, res.Result)
	require.True(t, s.chunkExists(123, 0), "legacy snapshot metadata must remain valid during rollout")
}

func TestApplySnapshotChunkRejectsUnsupportedCoreHistoryMode(t *testing.T) {
	s := &Server{
		config: &config.Config{
			RootDir:     t.TempDir(),
			GenesisFile: &cometbfttypes.GenesisDoc{ChainID: "audius-test"},
		},
		logger: zap.NewNop(),
	}

	metadata, err := json.Marshal(Metadata{
		Sender:  "validator-a",
		ChainID: "audius-test",
		CoreHistory: &CoreHistoryMetadata{
			Mode: "partitioned_epoch_archive",
		},
	})
	require.NoError(t, err)

	require.NoError(t, s.StoreOfferedSnapshot(&abciapi.Snapshot{
		Height:   123,
		Format:   1,
		Chunks:   1,
		Hash:     []byte("snapshot-hash"),
		Metadata: metadata,
	}))

	res, err := s.ApplySnapshotChunk(context.Background(), &abcitypes.ApplySnapshotChunkRequest{
		Index:  0,
		Sender: "validator-a",
		Chunk:  []byte("chunk-data"),
	})
	require.NoError(t, err)
	require.Equal(t, abcitypes.APPLY_SNAPSHOT_CHUNK_RESULT_REJECT_SNAPSHOT, res.Result)
	require.False(t, s.chunkExists(123, 0), "invalid snapshot metadata must be rejected before storing chunks")
}
