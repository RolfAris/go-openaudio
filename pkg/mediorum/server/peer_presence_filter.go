package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// peerPresenceFilter is a compact probabilistic summary of a peer's bucket
// contents, allowing findNodeToServeBlob to skip hosts that definitely don't
// have a blob without issuing a remote HeadObject.
type peerPresenceFilter struct {
	Filter     *bloom.BloomFilter `json:"filter"`
	EntryCount int                `json:"entryCount"`
	BuiltAt    time.Time          `json:"builtAt"`
}

// buildLocalPresenceFilter serializes the repair presence index into a Bloom
// filter for sharing with peers.
func (ss *MediorumServer) buildLocalPresenceFilter(index *repairPresenceIndex) {
	n := uint(len(index.entries))
	if n == 0 {
		return
	}
	f := bloom.NewWithEstimates(n, 0.001)
	for key := range index.entries {
		f.AddString(key)
	}

	ss.peerFiltersMutex.Lock()
	ss.localPresenceFilter = &peerPresenceFilter{
		Filter:     f,
		EntryCount: int(n),
		BuiltAt:    time.Now(),
	}
	ss.peerFiltersMutex.Unlock()

	ss.logger.Info("local presence filter built",
		zap.Int("entries", int(n)))
}

func (ss *MediorumServer) servePresenceFilter(c echo.Context) error {
	ss.peerFiltersMutex.RLock()
	pf := ss.localPresenceFilter
	ss.peerFiltersMutex.RUnlock()

	if pf == nil || pf.Filter == nil {
		return c.String(http.StatusNoContent, "no filter available")
	}
	return c.JSON(http.StatusOK, pf)
}

// peerMayHaveBlob checks the cached Bloom filter for a peer. Returns true if
// the blob might be present (or if no filter is available).
func (ss *MediorumServer) peerMayHaveBlob(host, key string) bool {
	ss.peerFiltersMutex.RLock()
	pf, ok := ss.peerFilters[host]
	ss.peerFiltersMutex.RUnlock()

	if !ok || pf == nil || pf.Filter == nil {
		return true
	}
	if time.Since(pf.BuiltAt) > 2*ss.Config.RepairInterval {
		return true
	}
	return pf.Filter.TestString(key)
}

func (ss *MediorumServer) startPresenceFilterPoller(ctx context.Context) error {
	// Wait for health data to populate before first fetch
	time.Sleep(2 * time.Minute)

	// Reuse peerHTTPClient's transport (handles TLS/self-signed) with a
	// shorter timeout since filter responses are <1 MB.
	client := &http.Client{
		Transport: ss.peerHTTPClient.Transport,
		Timeout:   10 * time.Second,
	}

	ss.fetchPeerFilters(client)

	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ss.fetchPeerFilters(client)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (ss *MediorumServer) fetchPeerFilters(client *http.Client) {
	peers := ss.findHealthyPeers(time.Hour)

	var wg sync.WaitGroup
	wg.Add(len(peers))

	for _, host := range peers {
		go func() {
			defer wg.Done()
			if host == ss.Config.Self.Host {
				return
			}

			url := apiPath(host, "internal/presence-filter")
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", "mediorum "+ss.Config.Self.Host)
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				return
			}

			body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB max
			if err != nil {
				return
			}

			var pf peerPresenceFilter
			if err := json.Unmarshal(body, &pf); err != nil {
				ss.logger.Warn("failed to unmarshal peer presence filter",
					zap.String("peer", host), zap.Error(err))
				return
			}

			ss.peerFiltersMutex.Lock()
			ss.peerFilters[host] = &pf
			ss.peerFiltersMutex.Unlock()
		}()
	}
	wg.Wait()
}
