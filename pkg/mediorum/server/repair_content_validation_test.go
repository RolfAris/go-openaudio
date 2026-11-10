package server

import (
	"bytes"
	"context"
	"fmt"
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

// Without a hash check, an existing archive object is not proof that its
// bytes can replace the good primary copy. Preserve that copy in skip mode.
func TestRepairCleanupWithoutValidationPreservesPrimaryArchiveCopy(t *testing.T) {
	ctx := context.Background()
	primary, archive := openMemBuckets(t)
	ss := makeArchiveEvictionServer(t, primary, archive, 4)
	ss.Config.SkipCleanupValidation = true
	var cid string
	var data []byte
	for i := 0; i < 1000; i++ {
		data = []byte(fmt.Sprintf("valid-primary-%d", i))
		var err error
		cid, err = cidutil.ComputeFileCID(bytes.NewReader(data))
		require.NoError(t, err)
		if ss.bucketForCID(cid, nil) == archive {
			break
		}
	}
	require.Equal(t, archive, ss.bucketForCID(cid, nil))
	key := cidutil.ShardCID(cid)
	require.NoError(t, primary.WriteAll(ctx, key, data, nil))
	require.NoError(t, archive.WriteAll(ctx, key, []byte("corrupt archive copy"), nil))
	tracker := newCleanupTracker(true)
	policy := newRepairRetentionPolicy(ss.Config, time.Now())
	require.NoError(t, ss.repairCidWithPolicy(ctx, cid, nil, tracker, nil, policy, time.Time{}))
	got, err := primary.ReadAll(ctx, key)
	require.NoError(t, err, "unvalidated archive must not replace the primary copy")
	assert.Equal(t, data, got)
	assert.Zero(t, tracker.Counters["archive_primary_duplicate_dropped"])
}
