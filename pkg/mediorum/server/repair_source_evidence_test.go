package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestRepairSourceEvidenceTrackerSnapshot(t *testing.T) {
	tracker := newRepairSourceEvidenceTracker()
	tracker.recordCall(repairSourceQmCID)
	tracker.recordFastSkipNotMine(repairSourceQmCID)
	tracker.recordCycle(repairSourceQmCID, false)
	tracker.recordCycle(repairSourceQmCID, true)
	tracker.recordAttrCall(repairSourceQmCID)
	tracker.recordShadowAttrCall(repairSourceQmCID)
	tracker.recordListIndexShadowMismatch(repairSourceQmCID)
	tracker.recordListIndexFallback(repairSourceQmCID)
	tracker.recordPullMineNeeded(repairSourceQmCID)

	snapshot := tracker.snapshot()
	assert.Equal(t, uint64(1), snapshot[repairSourceQmCID].CallsTotal)
	assert.Equal(t, uint64(1), snapshot[repairSourceQmCID].FastSkipNotMineTotal)
	assert.Equal(t, uint64(2), snapshot[repairSourceQmCID].CycleTotal)
	assert.Equal(t, uint64(1), snapshot[repairSourceQmCID].CycleUnique)
	assert.Equal(t, uint64(1), snapshot[repairSourceQmCID].CycleDuplicate)
	assert.Equal(t, uint64(1), snapshot[repairSourceQmCID].AttrCallsTotal)
	assert.Equal(t, uint64(1), snapshot[repairSourceQmCID].ShadowAttrCallsTotal)
	assert.Equal(t, uint64(1), snapshot[repairSourceQmCID].ListIndexShadowMismatchTotal)
	assert.Equal(t, uint64(1), snapshot[repairSourceQmCID].ListIndexFallbackTotal)
	assert.Equal(t, uint64(1), snapshot[repairSourceQmCID].PullMineNeededTotal)
}

func TestGetMetricsIncludesRepairSourceEvidence(t *testing.T) {
	ss := testNetwork[0]
	ss.repairSourceEvidence.recordCall(repairSourceUploadOrig)
	ss.repairSourceEvidence.recordAttrCall(repairSourceUploadOrig)

	req := httptest.NewRequest("GET", "/internal/metrics", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)

	assert.NoError(t, ss.getMetrics(c))
	assert.Equal(t, 200, rec.Code)

	var payload map[string]any
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Contains(t, payload, "storage_repair_source_evidence")
}
