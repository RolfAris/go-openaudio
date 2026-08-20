package server

import (
	"bytes"
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenAudio/go-openaudio/pkg/mediorum/cidutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepairCleanupContentValidationControl(t *testing.T) {
	ctx := context.Background()
	ss := testNetwork[0]

	data := []byte("cleanup-content-validation-control")
	cid, err := cidutil.ComputeFileCID(bytes.NewReader(data))
	require.NoError(t, err)
	require.NoError(t, ss.replicateToMyBucket(ctx, cid, bytes.NewReader(data), nil))
	t.Cleanup(func() { require.NoError(t, ss.dropFromMyBucket(cid)) })

	storeURL, err := url.Parse(ss.Config.BlobStoreDSN)
	require.NoError(t, err)
	path := filepath.Join(storeURL.Path, filepath.FromSlash(cidutil.ShardCID(cid)))
	require.NoError(t, os.WriteFile(path, []byte("corrupt-cleanup-body"), 0o644))

	ss.Config.SkipCleanupValidation = true
	t.Cleanup(func() { ss.Config.SkipCleanupValidation = false })

	disabledTracker := &RepairTracker{
		StartedAt:   time.Now(),
		CleanupMode: true,
		Counters:    map[string]int{},
	}
	require.NoError(t, ss.repairCid(ctx, cid, []string{ss.Config.Self.Host}, disabledTracker, nil))
	assert.True(t, ss.haveInMyBucket(cid))
	assert.Zero(t, disabledTracker.Counters["delete_invalid_success"])

	ss.Config.SkipCleanupValidation = false
	enabledTracker := &RepairTracker{
		StartedAt:   time.Now(),
		CleanupMode: true,
		Counters:    map[string]int{},
	}
	require.NoError(t, ss.repairCid(ctx, cid, []string{ss.Config.Self.Host}, enabledTracker, nil))
	assert.False(t, ss.haveInMyBucket(cid))
	assert.Equal(t, 1, enabledTracker.Counters["delete_invalid_success"])
}
