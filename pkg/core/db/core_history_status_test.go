package db

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestEstimateCoreHistoryBelowRetainFloor(t *testing.T) {
	minHeight := int64(1)
	maxHeight := int64(10)

	rows, bytes := estimateCoreHistoryBelowRetainFloor(&minHeight, &maxHeight, 100, 1000, 6)
	require.NotNil(t, rows)
	require.NotNil(t, bytes)
	require.EqualValues(t, 50, *rows)
	require.EqualValues(t, 500, *bytes)

	rows, bytes = estimateCoreHistoryBelowRetainFloor(&minHeight, &maxHeight, 100, 1000, 1)
	require.NotNil(t, rows)
	require.NotNil(t, bytes)
	require.EqualValues(t, 0, *rows)
	require.EqualValues(t, 0, *bytes)

	rows, bytes = estimateCoreHistoryBelowRetainFloor(&minHeight, &maxHeight, 100, 1000, 11)
	require.NotNil(t, rows)
	require.NotNil(t, bytes)
	require.EqualValues(t, 100, *rows)
	require.EqualValues(t, 1000, *bytes)
}

func TestGetCoreHistoryStatusReportsIndexedBounds(t *testing.T) {
	dsn := os.Getenv("TEST_DB_URL")
	if dsn == "" {
		t.Skip("TEST_DB_URL not set")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close(ctx))
	})

	_, err = conn.Exec(ctx, `
		create temporary table core_blocks(height bigint);
			create temporary table core_transactions(block_id bigint);
			create temporary table core_app_state(block_height bigint);
			create temporary table core_tx_stats(block_height bigint);
			create index on core_blocks(height);
			create index on core_transactions(block_id);
			create index on core_app_state(block_height);

			insert into core_blocks select generate_series(1, 10);
		insert into core_transactions select generate_series(1, 10);
		insert into core_app_state select generate_series(1, 10);
		insert into core_tx_stats select generate_series(1, 10);

		analyze core_blocks;
		analyze core_transactions;
		analyze core_app_state;
		analyze core_tx_stats;
	`)
	require.NoError(t, err)

	status, err := New(conn).GetCoreHistoryStatus(ctx, 6)
	require.NoError(t, err)
	require.EqualValues(t, 6, status.RetainFloorHeight)
	require.Len(t, status.Tables, 4)
	require.False(t, status.EstimatedBytesBelowRetainFloorKnown)
	require.Positive(t, status.TotalRelationBytes)
	require.Positive(t, status.EstimatedBytesBelowRetainFloor)

	blocks := requireCoreHistoryTableStatus(t, status, "core_blocks")
	require.True(t, blocks.HeightBoundsIndexed)
	require.True(t, blocks.Exists)
	require.EqualValues(t, 10, blocks.EstimatedRows)
	require.EqualValues(t, 1, *blocks.MinHeight)
	require.EqualValues(t, 10, *blocks.MaxHeight)
	require.EqualValues(t, 5, *blocks.EstimatedRowsBelowRetainFloor)
	require.Positive(t, *blocks.EstimatedBytesBelowRetainFloor)

	txStats := requireCoreHistoryTableStatus(t, status, "core_tx_stats")
	require.False(t, txStats.HeightBoundsIndexed)
	require.Nil(t, txStats.MinHeight)
	require.Nil(t, txStats.EstimatedRowsBelowRetainFloor)
	require.Contains(t, txStats.BelowRetainFloorEstimateUnavailable, "not indexed")
}

func requireCoreHistoryTableStatus(t *testing.T, status *CoreHistoryStatus, name string) CoreHistoryTableStatus {
	t.Helper()

	for _, table := range status.Tables {
		if table.Name == name {
			return table
		}
	}
	t.Fatalf("missing core history table status for %s", name)
	return CoreHistoryTableStatus{}
}
