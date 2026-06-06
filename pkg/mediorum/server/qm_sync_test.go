package server

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gocloud.dev/blob"
	"gocloud.dev/blob/driver"
	"gocloud.dev/gcerrors"
)

func TestQmSync(t *testing.T) {
	ctx := context.Background()

	ss := testNetwork[0]

	_, err := ss.pgPool.Exec(ctx, `insert into qm_cids values ('Qm1'), ('Qm2'), ('Qm3') on conflict do nothing`)
	assert.NoError(t, err)
	err = ss.writeQmFile()
	assert.NoError(t, err)

	// read it back out
	blobReader, err := ss.bucket.NewReader(ctx, _qmFileKey, nil)
	assert.NoError(t, err)

	cool, err := io.ReadAll(blobReader)
	assert.NoError(t, err)
	assert.Equal(t, "Qm1 Qm2 Qm3 ", strings.ReplaceAll(string(cool), "\n", " "))

	s2 := testNetwork[1]

	_, err = s2.pgPool.Exec(ctx, "truncate qm_cids, qm_sync")
	assert.NoError(t, err)

	s2count := -1
	s2.pgPool.QueryRow(ctx, "select count(*) from qm_cids").Scan(&s2count)
	assert.Equal(t, 0, s2count)

	s2done := false
	s2.pgPool.QueryRow(ctx, "select count(*) = 1 from qm_sync where host = $1", ss.Config.Self.Host).Scan(&s2done)
	assert.False(t, s2done)

	err = s2.pullQmFromPeer(ctx, ss.Config.Self.Host)
	assert.NoError(t, err)

	s2.pgPool.QueryRow(ctx, "select count(*) from qm_cids").Scan(&s2count)
	assert.Equal(t, 3, s2count)

	s2.pgPool.QueryRow(ctx, "select count(*) = 1 from qm_sync where host = $1", ss.Config.Self.Host).Scan(&s2done)
	assert.True(t, s2done)

	var qm string
	s2.pgPool.QueryRow(ctx, "select * from qm_cids order by key").Scan(&qm)
	assert.Equal(t, "Qm1", qm)

	// run it again
	err = s2.pullQmFromPeer(ctx, ss.Config.Self.Host)
	assert.NoError(t, err)

	// force duplicate run
	s2.pgPool.Exec(ctx, "truncate qm_sync")
	err = s2.pullQmFromPeer(ctx, ss.Config.Self.Host)
	assert.NoError(t, err)
}

func TestWriteQmFileClosesBlobWriterWhenCopyFails(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("QM_SYNC_MISSING_TABLE_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:example@localhost:5454/postgres"
	}
	poolConfig, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "set search_path to pg_temp")
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	fake := &closeCountingBucket{}
	ss := &MediorumServer{
		bucket: blob.NewBucket(fake),
		pgPool: pool,
		logger: zap.NewNop(),
	}

	err = ss.writeQmFile()
	require.Error(t, err)
	require.Contains(t, err.Error(), "qm_cids")
	require.Equal(t, int32(1), fake.writer.closeCount.Load())
	require.Equal(t, int32(1), fake.deleteCount.Load())
}

type closeCountingBucket struct {
	writer      closeCountingWriter
	deleteCount atomic.Int32
}

func (b *closeCountingBucket) ErrorCode(err error) gcerrors.ErrorCode {
	if errors.Is(err, errFakeBlobNotFound) {
		return gcerrors.NotFound
	}
	return gcerrors.Unknown
}

func (b *closeCountingBucket) As(interface{}) bool {
	return false
}

func (b *closeCountingBucket) ErrorAs(error, interface{}) bool {
	return false
}

func (b *closeCountingBucket) Attributes(context.Context, string) (*driver.Attributes, error) {
	return nil, errFakeBlobNotFound
}

func (b *closeCountingBucket) ListPaged(context.Context, *driver.ListOptions) (*driver.ListPage, error) {
	return nil, errors.New("not implemented")
}

func (b *closeCountingBucket) NewRangeReader(context.Context, string, int64, int64, *driver.ReaderOptions) (driver.Reader, error) {
	return nil, errFakeBlobNotFound
}

func (b *closeCountingBucket) NewTypedWriter(context.Context, string, string, *driver.WriterOptions) (driver.Writer, error) {
	return &b.writer, nil
}

func (b *closeCountingBucket) Copy(context.Context, string, string, *driver.CopyOptions) error {
	return errors.New("not implemented")
}

func (b *closeCountingBucket) Delete(context.Context, string) error {
	b.deleteCount.Add(1)
	return nil
}

func (b *closeCountingBucket) SignedURL(context.Context, string, *driver.SignedURLOptions) (string, error) {
	return "", errors.New("not implemented")
}

func (b *closeCountingBucket) Close() error {
	return nil
}

type closeCountingWriter struct {
	closeCount atomic.Int32
}

func (w *closeCountingWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (w *closeCountingWriter) Close() error {
	w.closeCount.Add(1)
	return nil
}

var errFakeBlobNotFound = errors.New("fake blob not found")
