package server

import "sync"

const (
	repairSourceUploadOrig      = "upload_orig"
	repairSourceUploadTranscode = "upload_transcode"
	repairSourceAudioPreview    = "audio_preview"
	repairSourceQmCID           = "qm_cids"
)

type repairSourceEvidence struct {
	CallsTotal            uint64 `json:"calls_total"`
	FastSkipNotMineTotal  uint64 `json:"fast_skip_not_mine_total"`
	CycleTotal            uint64 `json:"cycle_total"`
	CycleUnique           uint64 `json:"cycle_unique"`
	CycleDuplicate        uint64 `json:"cycle_duplicate"`
	KnownPresentHitsTotal uint64 `json:"known_present_hits_total"`
	AttrCallsTotal        uint64 `json:"attr_calls_total"`
	PullMineNeededTotal   uint64 `json:"pull_mine_needed_total"`
}

type repairSourceEvidenceTracker struct {
	mu      sync.Mutex
	sources map[string]*repairSourceEvidence
}

func newRepairSourceEvidenceTracker() *repairSourceEvidenceTracker {
	return &repairSourceEvidenceTracker{
		sources: map[string]*repairSourceEvidence{
			repairSourceUploadOrig:      {},
			repairSourceUploadTranscode: {},
			repairSourceAudioPreview:    {},
			repairSourceQmCID:           {},
		},
	}
}

func (t *repairSourceEvidenceTracker) sourceState(source string) *repairSourceEvidence {
	state, ok := t.sources[source]
	if !ok {
		state = &repairSourceEvidence{}
		t.sources[source] = state
	}
	return state
}

func (t *repairSourceEvidenceTracker) recordCall(source string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sourceState(source).CallsTotal++
}

func (t *repairSourceEvidenceTracker) recordFastSkipNotMine(source string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sourceState(source).FastSkipNotMineTotal++
}

func (t *repairSourceEvidenceTracker) recordCycle(source string, duplicate bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.sourceState(source)
	state.CycleTotal++
	if duplicate {
		state.CycleDuplicate++
		return
	}
	state.CycleUnique++
}

func (t *repairSourceEvidenceTracker) recordKnownPresentHit(source string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sourceState(source).KnownPresentHitsTotal++
}

func (t *repairSourceEvidenceTracker) recordAttrCall(source string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sourceState(source).AttrCallsTotal++
}

func (t *repairSourceEvidenceTracker) recordPullMineNeeded(source string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sourceState(source).PullMineNeededTotal++
}

func (t *repairSourceEvidenceTracker) snapshot() map[string]repairSourceEvidence {
	t.mu.Lock()
	defer t.mu.Unlock()
	payload := make(map[string]repairSourceEvidence, len(t.sources))
	for source, state := range t.sources {
		payload[source] = *state
	}
	return payload
}
