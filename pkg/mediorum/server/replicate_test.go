package server

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/OpenAudio/go-openaudio/pkg/mediorum/cidutil"
	"github.com/stretchr/testify/assert"
)

// errAfterNReader returns n bytes of data then a non-EOF error, simulating
// a network drop, source EOF, or context cancel mid-stream.
type errAfterNReader struct {
	remaining int
}

func (r *errAfterNReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, errors.New("simulated mid-stream failure")
	}
	n := len(p)
	if n > r.remaining {
		n = r.remaining
	}
	for i := 0; i < n; i++ {
		p[i] = 'x'
	}
	r.remaining -= n
	return n, nil
}

// TestReplicateToMyBucket_AbortsOnMidStreamError guards against a regression
// where an io.Copy error in replicateToMyBucket abandons the writer without
// calling Close(). Without the explicit Close on the error path the s3blob
// driver never issues AbortMultipartUpload, and on Backblaze B2 the
// b2_start_large_file call leaks as a stuck upload.
func TestReplicateToMyBucket_AbortsOnMidStreamError(t *testing.T) {
	ctx := context.Background()
	ss := testNetwork[0]

	cid, err := cidutil.ComputeFileCID(bytes.NewReader([]byte("stuck-upload-fix-test")))
	assert.NoError(t, err)

	err = ss.replicateToMyBucket(ctx, cid, &errAfterNReader{remaining: 64})
	assert.Error(t, err, "mid-stream read error must propagate")

	key := cidutil.ShardCID(cid)
	exists, _ := ss.bucket.Exists(ctx, key)
	assert.False(t, exists, "no partial blob should be committed when the writer errors")
}
