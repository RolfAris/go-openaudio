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

// TestReplicateToMyBucket_AbortsOnMidStreamError exercises the io.Copy
// error path in replicateToMyBucket. It asserts the error propagates and
// that no partial blob is committed.
//
// On the test fixture's fileblob backend an abandoned Writer leaves no
// committed blob whether or not Close() is called, so this test alone
// cannot prove Close() ran. The fix's real value is on the s3blob
// driver, where Close() after a write error issues AbortMultipartUpload
// — without that call B2's b2_start_large_file leaks as a stuck upload.
// End-to-end verification is via fleet-side B2 stuck-upload counts; this
// test guards the behavior contract (error propagates cleanly, no
// partial commit) so the call-site shape doesn't regress.
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
