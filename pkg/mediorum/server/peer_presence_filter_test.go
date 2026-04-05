package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/OpenAudio/go-openaudio/pkg/mediorum/cidutil"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestBuildLocalPresenceFilter(t *testing.T) {
	ctx := context.Background()
	ss := testNetwork[0]

	data := []byte("bloom-filter-test-blob")
	cid, err := cidutil.ComputeFileCID(bytes.NewReader(data))
	assert.NoError(t, err)
	assert.NoError(t, ss.replicateToMyBucket(ctx, cid, bytes.NewReader(data)))

	index, err := ss.buildRepairPresenceIndex(ctx)
	assert.NoError(t, err)

	ss.buildLocalPresenceFilter(index)

	ss.peerFiltersMutex.RLock()
	pf := ss.localPresenceFilter
	ss.peerFiltersMutex.RUnlock()

	t.Run("filter is built", func(t *testing.T) {
		assert.NotNil(t, pf)
		assert.NotNil(t, pf.Filter)
		assert.Greater(t, pf.EntryCount, 0)
	})

	t.Run("known key is present in filter", func(t *testing.T) {
		key := cidutil.ShardCID(cid)
		assert.True(t, pf.Filter.TestString(key))
	})

	t.Run("random key is absent from filter", func(t *testing.T) {
		assert.False(t, pf.Filter.TestString("nonexistent/key/that/should/not/match"))
	})
}

func TestBuildLocalPresenceFilterEmptyIndex(t *testing.T) {
	ss := testNetwork[1]

	empty := &repairPresenceIndex{entries: map[string]presenceEntry{}}
	ss.buildLocalPresenceFilter(empty)

	// Empty index should not overwrite any existing filter
	// (the function returns early for n == 0)
}

func TestPeerMayHaveBlob(t *testing.T) {
	ss := testNetwork[0]

	// Build a filter with enough entries for meaningful FPR. The auto-built
	// filter from the test bucket is too small (1-2 entries = all bits set).
	knownKey := "ab/baeaaaiqknownkeythatexistsinfilter"

	filter := bloom.NewWithEstimates(1000, 0.001)
	filter.AddString(knownKey)
	// Add some filler keys so the filter is realistic
	for i := 0; i < 999; i++ {
		filter.AddString(fmt.Sprintf("fi/%d-filler-key-for-bloom-test", i))
	}

	pf := &peerPresenceFilter{
		Filter:     filter,
		EntryCount: 1000,
		BuiltAt:    time.Now(),
	}

	// Ensure RepairInterval is nonzero so the staleness check works
	prevInterval := ss.Config.RepairInterval
	ss.Config.RepairInterval = time.Hour
	defer func() { ss.Config.RepairInterval = prevInterval }()

	fakeHost := "http://fake-peer:1999"
	ss.peerFiltersMutex.Lock()
	ss.peerFilters[fakeHost] = pf
	ss.peerFiltersMutex.Unlock()
	defer func() {
		ss.peerFiltersMutex.Lock()
		delete(ss.peerFilters, fakeHost)
		ss.peerFiltersMutex.Unlock()
	}()

	t.Run("returns true for key in filter", func(t *testing.T) {
		assert.True(t, ss.peerMayHaveBlob(fakeHost, knownKey))
	})

	t.Run("rejects most absent keys", func(t *testing.T) {
		falsePositives := 0
		trials := 100
		for i := 0; i < trials; i++ {
			k := fmt.Sprintf("zz/%d-absent-key-should-not-be-in-filter", i)
			if ss.peerMayHaveBlob(fakeHost, k) {
				falsePositives++
			}
		}
		// With 0.1% FPR, expect ~0.1 false positives out of 100 trials.
		// Allow up to 5 to account for statistical noise.
		assert.Less(t, falsePositives, 5, "too many false positives: %d/%d", falsePositives, trials)
	})

	t.Run("returns true for unknown host", func(t *testing.T) {
		assert.True(t, ss.peerMayHaveBlob("http://unknown:9999", knownKey))
	})

	t.Run("returns true for stale filter", func(t *testing.T) {
		ss.peerFiltersMutex.Lock()
		ss.peerFilters[fakeHost] = &peerPresenceFilter{
			Filter:     pf.Filter,
			EntryCount: pf.EntryCount,
			BuiltAt:    time.Now().Add(-48 * time.Hour),
		}
		ss.peerFiltersMutex.Unlock()

		// Stale filter → assume present (safe default), even for absent keys
		assert.True(t, ss.peerMayHaveBlob(fakeHost, "zz/definitely-absent-stale-test"))

		ss.peerFiltersMutex.Lock()
		ss.peerFilters[fakeHost] = pf
		ss.peerFiltersMutex.Unlock()
	})
}

func TestServePresenceFilter(t *testing.T) {
	ctx := context.Background()
	ss := testNetwork[0]

	data := []byte("serve-filter-test")
	cid, err := cidutil.ComputeFileCID(bytes.NewReader(data))
	assert.NoError(t, err)
	assert.NoError(t, ss.replicateToMyBucket(ctx, cid, bytes.NewReader(data)))

	index, err := ss.buildRepairPresenceIndex(ctx)
	assert.NoError(t, err)
	ss.buildLocalPresenceFilter(index)

	t.Run("returns filter as json", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/internal/presence-filter", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := ss.servePresenceFilter(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var pf peerPresenceFilter
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pf))
		assert.Greater(t, pf.EntryCount, 0)
		assert.NotNil(t, pf.Filter)
	})

	t.Run("filter round-trips through json", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/internal/presence-filter", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		assert.NoError(t, ss.servePresenceFilter(c))

		var pf peerPresenceFilter
		assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pf))

		key := cidutil.ShardCID(cid)
		assert.True(t, pf.Filter.TestString(key))
		assert.False(t, pf.Filter.TestString("nonexistent/key"))
	})
}

func TestServePresenceFilterNoFilter(t *testing.T) {
	ss := testNetwork[2]

	// Clear any existing filter
	ss.peerFiltersMutex.Lock()
	prev := ss.localPresenceFilter
	ss.localPresenceFilter = nil
	ss.peerFiltersMutex.Unlock()
	defer func() {
		ss.peerFiltersMutex.Lock()
		ss.localPresenceFilter = prev
		ss.peerFiltersMutex.Unlock()
	}()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/internal/presence-filter", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = ss.servePresenceFilter(c)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}
