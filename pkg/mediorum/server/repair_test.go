package server

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/OpenAudio/go-openaudio/pkg/mediorum/cidutil"

	"github.com/stretchr/testify/assert"
)

func testNetworkRunRepair(cleanup bool) {
	wg := sync.WaitGroup{}
	wg.Add(len(testNetwork))
	for _, s := range testNetwork {
		s := s
		go func() {
			err := s.runRepair(context.Background(), &RepairTracker{StartedAt: time.Now(), CleanupMode: cleanup, Counters: map[string]int{}})
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func testNetworkLocateBlob(cid string) []string {
	ctx := context.Background()
	key := cidutil.ShardCID(cid)
	result := []string{}
	for _, s := range testNetwork {
		if ok, _ := s.bucket.Exists(ctx, key); ok {
			result = append(result, s.Config.Self.Host)
		}
	}
	return result
}

func TestRepair(t *testing.T) {
	ctx := context.Background()
	replicationFactor := 5

	ss := testNetwork[0]

	// first, write a blob only to my storage
	data := []byte("repair test")
	cid, err := cidutil.ComputeFileCID(bytes.NewReader(data))
	assert.NoError(t, err)
	err = ss.replicateToMyBucket(ctx, cid, bytes.NewReader(data))
	assert.NoError(t, err)

	// create a dummy upload for it?
	ss.crud.Create(Upload{
		ID:          "testing",
		OrigFileCID: cid,
		CreatedAt:   time.Now(),
	})

	// verify we can get it "manually"
	{
		s2 := testNetwork[1]
		u, err := s2.peerGetUpload(ss.Config.Self.Host, "testing")
		assert.NoError(t, err)
		assert.Equal(t, cid, u.OrigFileCID)

		var uploads []Upload
		resp, err := s2.reqClient.R().SetSuccessResult(&uploads).Get(ss.Config.Self.Host + "/uploads")
		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Len(t, uploads, 1)
		assert.NotEmpty(t, resp.GetHeader("x-took"))
	}

	// force sweep (since blob changes SkipBroadcast)
	for _, s := range testNetwork {
		s.crud.ForceSweep()
	}

	// assert it only exists on 1 host
	{
		hosts := testNetworkLocateBlob(cid)
		assert.Len(t, hosts, 1)
	}

	// tell all servers do repair
	testNetworkRunRepair(true)

	// assert it exists on R hosts
	{
		hosts := testNetworkLocateBlob(cid)

		// cleanup will permit blob on R+2
		// so assert upper threshold thusly
		assert.LessOrEqual(t, len(hosts), replicationFactor+2)
	}

	// --------------------------
	//
	// now over-replicate file
	//
	for _, server := range testNetwork {
		ss.replicateFileToHost(ctx, server.Config.Self.Host, cid, bytes.NewReader(data))
	}

	// assert over-replicated
	{
		hosts := testNetworkLocateBlob(cid)
		assert.Len(t, hosts, len(testNetwork))
	}

	// tell all servers do cleanup
	testNetworkRunRepair(true)

	// assert R copies
	if false {
		hosts := testNetworkLocateBlob(cid)
		assert.Len(t, hosts, replicationFactor)
	}

	// ----------------------
	// now make one of the servers "lose" a file
	if false {
		byHost := map[string]*MediorumServer{}
		for _, s := range testNetwork {
			byHost[s.Config.Self.Host] = s
		}

		rendezvousOrder := []*MediorumServer{}
		preferred, _ := ss.rendezvousAllHosts(cid)
		for _, h := range preferred {
			rendezvousOrder = append(rendezvousOrder, byHost[h])
		}

		// make leader lose file
		leader := rendezvousOrder[0]
		leader.dropFromMyBucket(cid)

		// normally a standby server wouldn't pull this file
		standby := rendezvousOrder[replicationFactor+2]
		err = standby.runRepair(ctx, &RepairTracker{StartedAt: time.Now(), CleanupMode: false, Counters: map[string]int{}})
		assert.NoError(t, err)
		assert.False(t, standby.hostHasBlob(standby.Config.Self.Host, cid))

		// running repair in cleanup mode... standby will observe that #1 doesn't have blob so will pull it
		err = standby.runRepair(ctx, &RepairTracker{StartedAt: time.Now(), CleanupMode: true, Counters: map[string]int{}})
		assert.NoError(t, err)
		assert.True(t, standby.hostHasBlob(standby.Config.Self.Host, cid))

		// leader re-gets lost file when repair runs
		err = leader.runRepair(ctx, &RepairTracker{StartedAt: time.Now(), CleanupMode: false, Counters: map[string]int{}})
		assert.NoError(t, err)
		assert.True(t, leader.hostHasBlob(leader.Config.Self.Host, cid))

		// standby drops file after leader has it back
		err = standby.runRepair(ctx, &RepairTracker{StartedAt: time.Now(), CleanupMode: true, Counters: map[string]int{}})
		assert.NoError(t, err)
		assert.False(t, standby.hostHasBlob(standby.Config.Self.Host, cid))
	}

}

func TestNextRepairCursorRespectsCleanupEvery(t *testing.T) {
	cursor, cleanup := nextRepairCursor(3, 4)
	assert.Equal(t, 4, cursor)
	assert.False(t, cleanup)

	cursor, cleanup = nextRepairCursor(4, 4)
	assert.Equal(t, 1, cursor)
	assert.True(t, cleanup)

	cursor, cleanup = nextRepairCursor(11, 12)
	assert.Equal(t, 12, cursor)
	assert.False(t, cleanup)

	cursor, cleanup = nextRepairCursor(12, 12)
	assert.Equal(t, 1, cursor)
	assert.True(t, cleanup)
}

func TestShouldRunQmCidsCleanup(t *testing.T) {
	assert.True(t, shouldRunQmCidsCleanup(1, 1))
	assert.True(t, shouldRunQmCidsCleanup(1, 12))
	assert.False(t, shouldRunQmCidsCleanup(2, 12))
	assert.False(t, shouldRunQmCidsCleanup(12, 12))
	assert.True(t, shouldRunQmCidsCleanup(13, 12))
	assert.False(t, shouldRunQmCidsCleanup(1, 0))
}

func TestRepairCidWithCleanupOverrideSkipsNotMineFastPath(t *testing.T) {
	ctx := context.Background()
	ss := testNetwork[0]

	var cid string
	for i := 0; i < 128; i++ {
		candidate, err := cidutil.ComputeFileCID(bytes.NewReader([]byte("qm-cleanup-fast-skip-" + string(rune('a'+i)))))
		assert.NoError(t, err)
		_, isMine := ss.rendezvousAllHosts(candidate)
		if !isMine {
			cid = candidate
			break
		}
	}
	if cid == "" {
		t.Fatal("failed to find cid that is not mine")
	}

	tracker := &RepairTracker{
		StartedAt:         time.Now(),
		CleanupMode:       true,
		QmCidsCleanupMode: false,
		Counters:          map[string]int{},
	}

	assert.NoError(t, ss.repairCidWithCleanupMode(ctx, cid, nil, tracker, repairSourceQmCID, false))
	assert.Equal(t, 0, tracker.Counters["total_checked"])

	snapshot := ss.repairSourceEvidence.snapshot()
	assert.Positive(t, snapshot[repairSourceQmCID].FastSkipNotMineTotal)
}

func TestBuildRepairPresenceIndexIncludesLocalBlob(t *testing.T) {
	ctx := context.Background()
	ss := testNetwork[0]

	data := []byte("presence-index-local-blob")
	cid, err := cidutil.ComputeFileCID(bytes.NewReader(data))
	assert.NoError(t, err)
	assert.NoError(t, ss.replicateToMyBucket(ctx, cid, bytes.NewReader(data)))

	index, err := ss.buildRepairPresenceIndex(ctx)
	assert.NoError(t, err)

	entry, ok := index.Lookup(cidutil.ShardCID(cid))
	assert.True(t, ok)
	assert.Equal(t, int64(len(data)), entry.Size)
}

func TestRepairCidWithPresenceIndexUsesListedState(t *testing.T) {
	ctx := context.Background()
	ss := testNetwork[0]

	data := []byte("presence-index-repair-path")
	cid, err := cidutil.ComputeFileCID(bytes.NewReader(data))
	assert.NoError(t, err)
	assert.NoError(t, ss.replicateToMyBucket(ctx, cid, bytes.NewReader(data)))

	index, err := ss.buildRepairPresenceIndex(ctx)
	assert.NoError(t, err)

	assert.NoError(t, ss.dropFromMyBucket(cid))

	before := ss.repairSourceEvidence.snapshot()[repairSourceQmCID].AttrCallsTotal
	tracker := &RepairTracker{
		StartedAt:         time.Now(),
		CleanupMode:       true,
		QmCidsCleanupMode: true,
		Counters:          map[string]int{},
	}

	assert.NoError(t, ss.repairCidWithPresenceIndex(ctx, cid, []string{ss.Config.Self.Host}, tracker, repairSourceQmCID, false, index))
	assert.Equal(t, 1, tracker.Counters["already_have"])
	assert.Equal(t, 1, tracker.Counters["qm_cids_list_index_lookup"])
	assert.Equal(t, 1, tracker.Counters["qm_cids_list_index_present"])

	after := ss.repairSourceEvidence.snapshot()[repairSourceQmCID].AttrCallsTotal
	assert.Equal(t, before, after)
}

func TestRepairCidWithPresenceIndexShadowMismatchFallsBack(t *testing.T) {
	ctx := context.Background()
	ss := testNetwork[0]

	var cid string
	var data []byte
	for i := 0; i < 128; i++ {
		candidateData := []byte("presence-index-shadow-mismatch-" + string(rune('a'+i)))
		candidateCID, err := cidutil.ComputeFileCID(bytes.NewReader(candidateData))
		assert.NoError(t, err)
		_, isMine := ss.rendezvousAllHosts(candidateCID)
		if !isMine {
			cid = candidateCID
			data = candidateData
			break
		}
	}
	if cid == "" {
		t.Fatal("failed to find cid that is not mine")
	}
	assert.NoError(t, ss.replicateToMyBucket(ctx, cid, bytes.NewReader(data)))

	index, err := ss.buildRepairPresenceIndex(ctx)
	assert.NoError(t, err)
	index.shadowCompareEvery = 1
	index.disableOnShadowMismatch = true

	assert.NoError(t, ss.dropFromMyBucket(cid))

	tracker := &RepairTracker{
		StartedAt:         time.Now(),
		CleanupMode:       true,
		QmCidsCleanupMode: true,
		Counters:          map[string]int{},
	}

	before := ss.repairSourceEvidence.snapshot()[repairSourceQmCID]
	assert.NoError(t, ss.repairCidWithPresenceIndex(ctx, cid, nil, tracker, repairSourceQmCID, true, index))

	after := ss.repairSourceEvidence.snapshot()[repairSourceQmCID]
	assert.True(t, index.ShouldUsePerKeyAttrsFallback())
	assert.Equal(t, 1, tracker.Counters["qm_cids_list_index_lookup"])
	assert.Equal(t, 1, tracker.Counters["qm_cids_list_index_present"])
	assert.Equal(t, 1, tracker.Counters["qm_cids_list_index_shadow_compare_total"])
	assert.Equal(t, 1, tracker.Counters["qm_cids_list_index_shadow_mismatch"])
	assert.Equal(t, 1, tracker.Counters["qm_cids_list_index_fallback_triggered"])
	assert.Equal(t, 0, tracker.Counters["pull_mine_needed"])
	assert.Equal(t, 0, tracker.Counters["already_have"])
	assert.Equal(t, before.AttrCallsTotal, after.AttrCallsTotal)
	assert.Equal(t, before.ShadowAttrCallsTotal+1, after.ShadowAttrCallsTotal)
	assert.Equal(t, before.ListIndexShadowMismatchTotal+1, after.ListIndexShadowMismatchTotal)
	assert.Equal(t, before.ListIndexFallbackTotal+1, after.ListIndexFallbackTotal)
}

func TestRepairCidWithPresenceIndexFallbackUsesPerKeyAttrsAfterMismatch(t *testing.T) {
	ctx := context.Background()
	ss := testNetwork[0]

	var staleCID string
	var staleData []byte
	var fallbackCID string
	for i := 0; i < 256; i++ {
		candidateData := []byte("presence-index-fallback-" + string(rune('a'+(i%26))) + string(rune('A'+(i/26))))
		candidateCID, err := cidutil.ComputeFileCID(bytes.NewReader(candidateData))
		assert.NoError(t, err)
		_, isMine := ss.rendezvousAllHosts(candidateCID)
		if isMine {
			continue
		}
		if staleCID == "" {
			staleCID = candidateCID
			staleData = candidateData
			continue
		}
		fallbackCID = candidateCID
		break
	}
	if staleCID == "" || fallbackCID == "" {
		t.Fatal("failed to find non-mine cids for fallback test")
	}

	assert.NoError(t, ss.replicateToMyBucket(ctx, staleCID, bytes.NewReader(staleData)))

	index, err := ss.buildRepairPresenceIndex(ctx)
	assert.NoError(t, err)
	index.shadowCompareEvery = 1
	index.disableOnShadowMismatch = true

	assert.NoError(t, ss.dropFromMyBucket(staleCID))

	before := ss.repairSourceEvidence.snapshot()[repairSourceQmCID]
	mismatchTracker := &RepairTracker{
		StartedAt:         time.Now(),
		CleanupMode:       true,
		QmCidsCleanupMode: true,
		Counters:          map[string]int{},
	}
	assert.NoError(t, ss.repairCidWithPresenceIndex(ctx, staleCID, nil, mismatchTracker, repairSourceQmCID, true, index))
	assert.True(t, index.ShouldUsePerKeyAttrsFallback())

	fallbackTracker := &RepairTracker{
		StartedAt:         time.Now(),
		CleanupMode:       true,
		QmCidsCleanupMode: true,
		Counters:          map[string]int{},
	}
	assert.NoError(t, ss.repairCidWithPresenceIndex(ctx, fallbackCID, nil, fallbackTracker, repairSourceQmCID, true, index))

	after := ss.repairSourceEvidence.snapshot()[repairSourceQmCID]
	assert.Equal(t, 1, fallbackTracker.Counters["qm_cids_list_index_fallback_lookup"])
	assert.Equal(t, 0, fallbackTracker.Counters["qm_cids_list_index_lookup"])
	assert.Equal(t, before.AttrCallsTotal+1, after.AttrCallsTotal)
	assert.Equal(t, before.ShadowAttrCallsTotal+1, after.ShadowAttrCallsTotal)
	assert.Equal(t, before.ListIndexShadowMismatchTotal+1, after.ListIndexShadowMismatchTotal)
	assert.Equal(t, before.ListIndexFallbackTotal+2, after.ListIndexFallbackTotal)
}

func TestRepairCidWithPresenceIndexShadowMatchStaysOnFastPath(t *testing.T) {
	ctx := context.Background()
	ss := testNetwork[0]

	data := []byte("presence-index-shadow-match")
	cid, err := cidutil.ComputeFileCID(bytes.NewReader(data))
	assert.NoError(t, err)
	assert.NoError(t, ss.replicateToMyBucket(ctx, cid, bytes.NewReader(data)))

	index, err := ss.buildRepairPresenceIndex(ctx)
	assert.NoError(t, err)
	index.shadowCompareEvery = 1
	index.disableOnShadowMismatch = true

	tracker := &RepairTracker{
		StartedAt:         time.Now(),
		CleanupMode:       true,
		QmCidsCleanupMode: true,
		Counters:          map[string]int{},
	}

	before := ss.repairSourceEvidence.snapshot()[repairSourceQmCID]
	assert.NoError(t, ss.repairCidWithPresenceIndex(ctx, cid, []string{ss.Config.Self.Host}, tracker, repairSourceQmCID, false, index))

	after := ss.repairSourceEvidence.snapshot()[repairSourceQmCID]
	assert.False(t, index.ShouldUsePerKeyAttrsFallback())
	assert.Equal(t, 1, tracker.Counters["qm_cids_list_index_shadow_compare_total"])
	assert.Equal(t, 1, tracker.Counters["qm_cids_list_index_shadow_match"])
	assert.Equal(t, before.AttrCallsTotal, after.AttrCallsTotal)
	assert.Equal(t, before.ShadowAttrCallsTotal+1, after.ShadowAttrCallsTotal)
	assert.Equal(t, before.ListIndexShadowMismatchTotal, after.ListIndexShadowMismatchTotal)
}

func TestRepairCidWithPresenceIndexToleratesSubsecondModTimeDrift(t *testing.T) {
	ctx := context.Background()
	ss := testNetwork[0]

	data := []byte("presence-index-modtime-tolerance")
	cid, err := cidutil.ComputeFileCID(bytes.NewReader(data))
	assert.NoError(t, err)
	assert.NoError(t, ss.replicateToMyBucket(ctx, cid, bytes.NewReader(data)))

	index, err := ss.buildRepairPresenceIndex(ctx)
	assert.NoError(t, err)
	index.shadowCompareEvery = 1
	index.disableOnShadowMismatch = true

	key := cidutil.ShardCID(cid)
	entry, ok := index.Lookup(key)
	assert.True(t, ok)
	entry.ModTime = entry.ModTime.Add(500 * time.Millisecond)
	index.entries[key] = entry

	tracker := &RepairTracker{
		StartedAt:         time.Now(),
		CleanupMode:       true,
		QmCidsCleanupMode: true,
		Counters:          map[string]int{},
	}

	before := ss.repairSourceEvidence.snapshot()[repairSourceQmCID]
	assert.NoError(t, ss.repairCidWithPresenceIndex(ctx, cid, []string{ss.Config.Self.Host}, tracker, repairSourceQmCID, false, index))

	after := ss.repairSourceEvidence.snapshot()[repairSourceQmCID]
	assert.False(t, index.ShouldUsePerKeyAttrsFallback())
	assert.Equal(t, 1, tracker.Counters["qm_cids_list_index_shadow_compare_total"])
	assert.Equal(t, 1, tracker.Counters["qm_cids_list_index_shadow_match"])
	assert.Equal(t, 0, tracker.Counters["qm_cids_list_index_shadow_mismatch"])
	assert.Equal(t, before.AttrCallsTotal, after.AttrCallsTotal)
	assert.Equal(t, before.ShadowAttrCallsTotal+1, after.ShadowAttrCallsTotal)
	assert.Equal(t, before.ListIndexShadowMismatchTotal, after.ListIndexShadowMismatchTotal)
}
