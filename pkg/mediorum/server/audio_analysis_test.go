package server

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFindMissedAudioAnalysisCandidatesSkipsTerminalTranscodeFailures(t *testing.T) {
	ctx := context.Background()
	ss := testNetwork[0]
	prefix := fmt.Sprintf("!!!!audio-analysis-transcode-limit-%d-", time.Now().UnixNano())

	cleanup := func() {
		require.NoError(t, ss.crud.DB.Where("id LIKE ?", prefix+"%").Delete(&Upload{}).Error)
	}
	cleanup()
	t.Cleanup(cleanup)

	uploads := []Upload{
		{
			ID:                  prefix + "ready",
			Template:            JobTemplateAudio,
			AudioAnalysisStatus: "",
			TranscodeResults:    map[string]string{},
		},
		{
			ID:                  prefix + "terminal-no-result",
			Template:            JobTemplateAudio,
			Status:              JobStatusError,
			ErrorCount:          missedTranscodeMaxErrorCount + 1,
			AudioAnalysisStatus: "",
			TranscodeResults:    map[string]string{},
		},
		{
			ID:                  prefix + "terminal-with-result",
			Template:            JobTemplateAudio,
			Status:              JobStatusError,
			ErrorCount:          missedTranscodeMaxErrorCount + 1,
			AudioAnalysisStatus: "",
			TranscodeResults:    map[string]string{"320": "cid-320"},
		},
		{
			ID:                  prefix + "done",
			Template:            JobTemplateAudio,
			AudioAnalysisStatus: JobStatusDone,
			TranscodeResults:    map[string]string{},
		},
	}
	for i := range uploads {
		require.NoError(t, ss.crud.DB.Create(&uploads[i]).Error)
	}

	candidates, err := ss.findMissedAudioAnalysisCandidates(ctx)
	require.NoError(t, err)

	got := map[string]bool{}
	for _, upload := range candidates {
		if strings.HasPrefix(upload.ID, prefix) {
			got[upload.ID] = true
		}
	}

	require.Equal(t, map[string]bool{
		prefix + "ready":                true,
		prefix + "terminal-with-result": true,
	}, got)
}
