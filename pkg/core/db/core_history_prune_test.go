package db

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestCoreHistoryPruneSkipsUnsafeFloors(t *testing.T) {
	q := New(panicDB{})

	result, err := q.PruneCoreHistory(context.Background(), 1, 100)
	require.NoError(t, err)
	require.True(t, result.Complete)
	require.Zero(t, result.DeletedRows)

	result, err = q.PruneCoreHistory(context.Background(), 100, 0)
	require.NoError(t, err)
	require.True(t, result.Complete)
	require.Zero(t, result.DeletedRows)
}

func TestCoreHistoryPruneSQLUsesIndexedHeightColumns(t *testing.T) {
	sql := coreHistoryPruneSQL(coreHistoryPruneTable{name: "core_tx_stats", heightColumn: "block_height"})

	require.Contains(t, sql, "delete from core_tx_stats")
	require.Contains(t, sql, "where block_height < $1")
	require.Contains(t, sql, "order by block_height")
	require.Contains(t, sql, "limit $2")
}

func TestCoreHistoryPruneRunsTablesInDependencyOrder(t *testing.T) {
	database := &coreHistoryPruneDB{counts: []int64{10, 5, 0, 3}}
	q := New(database)

	result, err := q.PruneCoreHistory(context.Background(), 80, 100)

	require.NoError(t, err)
	require.True(t, result.Complete)
	require.EqualValues(t, 18, result.DeletedRows)
	require.Equal(t, []string{"core_transactions", "core_tx_stats", "core_app_state", "core_blocks"}, database.tables)
	require.Equal(t, []CoreHistoryPruneTableResult{
		{Name: "core_transactions", DeletedRows: 10},
		{Name: "core_tx_stats", DeletedRows: 5},
		{Name: "core_app_state", DeletedRows: 0},
		{Name: "core_blocks", DeletedRows: 3},
	}, result.Tables)
}

func TestCoreHistoryPruneMarksIncompleteWhenBatchLimitIsReached(t *testing.T) {
	database := &coreHistoryPruneDB{counts: []int64{100, 0, 0, 0}}
	q := New(database)

	result, err := q.PruneCoreHistory(context.Background(), 80, 100)

	require.NoError(t, err)
	require.False(t, result.Complete)
	require.EqualValues(t, 100, result.DeletedRows)
}

type panicDB struct{}

func (panicDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	panic("unexpected Exec")
}

func (panicDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	panic("unexpected Query")
}

func (panicDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	panic("unexpected QueryRow")
}

type coreHistoryPruneDB struct {
	counts []int64
	tables []string
}

func (d *coreHistoryPruneDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	panic("unexpected Exec")
}

func (d *coreHistoryPruneDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	panic("unexpected Query")
}

func (d *coreHistoryPruneDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	for _, table := range []string{"core_transactions", "core_tx_stats", "core_app_state", "core_blocks"} {
		if strings.Contains(query, "delete from "+table) {
			d.tables = append(d.tables, table)
			break
		}
	}

	count := d.counts[0]
	d.counts = d.counts[1:]
	return coreHistoryPruneRow{count: count}
}

type coreHistoryPruneRow struct {
	count int64
}

func (r coreHistoryPruneRow) Scan(dest ...interface{}) error {
	*(dest[0].(*int64)) = r.count
	return nil
}
