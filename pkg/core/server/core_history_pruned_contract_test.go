package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/OpenAudio/go-openaudio/pkg/core/config"
	coredb "github.com/OpenAudio/go-openaudio/pkg/core/db"
	"github.com/cometbft/cometbft/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestCoreHistoryIsBelowRetainFloorUsesLastRetainHeight(t *testing.T) {
	s := &Server{
		config: &config.Config{RetainHeight: 100},
		cache:  &Cache{},
		abciState: &ABCIState{
			lastRetainHeight: 900,
		},
	}
	s.cache.currentHeight.Store(1_000)

	retainFloor, belowFloor := s.coreHistoryIsBelowRetainFloor(899)
	require.Equal(t, int64(900), retainFloor)
	require.True(t, belowFloor)

	_, belowFloor = s.coreHistoryIsBelowRetainFloor(900)
	require.False(t, belowFloor)

	_, belowFloor = s.coreHistoryIsBelowRetainFloor(901)
	require.False(t, belowFloor)
}

func TestCoreHistoryIsBelowRetainFloorFallsBackToCurrentHeightMinusWindow(t *testing.T) {
	s := &Server{
		config:    &config.Config{RetainHeight: 100},
		cache:     &Cache{},
		abciState: &ABCIState{},
	}
	s.cache.currentHeight.Store(1_000)

	retainFloor, belowFloor := s.coreHistoryIsBelowRetainFloor(899)
	require.Equal(t, int64(900), retainFloor)
	require.True(t, belowFloor)
}

func TestCoreHistoryIsBelowRetainFloorDisabledForArchiveNodes(t *testing.T) {
	s := &Server{
		config: &config.Config{
			Archive:      true,
			RetainHeight: 100,
		},
		cache: &Cache{},
	}
	s.cache.currentHeight.Store(1_000)

	retainFloor, belowFloor := s.coreHistoryIsBelowRetainFloor(1)
	require.Zero(t, retainFloor)
	require.False(t, belowFloor)
}

func TestCoreHistoryPrunedErrorReturnsNotFoundBelowRetainFloor(t *testing.T) {
	s := &Server{
		config:    &config.Config{RetainHeight: 100},
		cache:     &Cache{},
		abciState: &ABCIState{},
	}
	s.cache.currentHeight.Store(1_000)
	c := &CoreService{core: s}

	err := c.coreHistoryPrunedError("block", 899)

	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.ErrorContains(t, err, "block at height 899 is below retained core history floor 900")
}

func TestCoreHistoryPrunedErrorReturnsNilAtOrAboveRetainFloor(t *testing.T) {
	s := &Server{
		config:    &config.Config{RetainHeight: 100},
		cache:     &Cache{},
		abciState: &ABCIState{},
	}
	s.cache.currentHeight.Store(1_000)
	c := &CoreService{core: s}

	require.NoError(t, c.coreHistoryPrunedError("block", 900))
	require.NoError(t, c.coreHistoryPrunedError("block", 901))
}

func TestGetBlockReturnsPrunedErrorBeforeRpcFallback(t *testing.T) {
	c := newCoreHistoryPrunedContractService(t, coreHistoryPrunedContractDB{})

	_, err := c.GetBlock(context.Background(), connect.NewRequest(&v1.GetBlockRequest{Height: 899}))

	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.ErrorContains(t, err, "block at height 899 is below retained core history floor 900")
}

func TestGetBlocksReturnsPrunedErrorForMissingBelowFloorBlock(t *testing.T) {
	c := newCoreHistoryPrunedContractService(t, coreHistoryPrunedContractDB{})

	_, err := c.GetBlocks(context.Background(), connect.NewRequest(&v1.GetBlocksRequest{
		Height: []int64{899},
	}))

	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.ErrorContains(t, err, "block at height 899 is below retained core history floor 900")
}

func TestGetTransactionReturnsPrunedErrorWhenBlockRowMissingBelowFloor(t *testing.T) {
	c := newCoreHistoryPrunedContractService(t, coreHistoryPrunedContractDB{
		transactionBlockID: 899,
	})

	_, err := c.GetTransaction(context.Background(), connect.NewRequest(&v1.GetTransactionRequest{
		TxHash: "0xold",
	}))

	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.ErrorContains(t, err, "transaction block at height 899 is below retained core history floor 900")
}

func TestGetTransactionReturnsNotFoundWhenTransactionRowPruned(t *testing.T) {
	c := newCoreHistoryPrunedContractService(t, coreHistoryPrunedContractDB{
		transactionMissing: true,
	})

	_, err := c.GetTransaction(context.Background(), connect.NewRequest(&v1.GetTransactionRequest{
		TxHash: "0xold",
	}))

	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.ErrorContains(t, err, "transaction 0xold not found in retained core history (retained floor 900)")
}

func newCoreHistoryPrunedContractService(t *testing.T, database coreHistoryPrunedContractDB) *CoreService {
	t.Helper()

	s := &Server{
		config: &config.Config{
			GenesisFile:  &types.GenesisDoc{ChainID: "test-chain"},
			RetainHeight: 100,
		},
		logger:    zap.NewNop(),
		db:        coredb.New(database),
		cache:     &Cache{},
		abciState: &ABCIState{},
	}
	s.cache.currentHeight.Store(1_000)

	return &CoreService{core: s}
}

type coreHistoryPrunedContractDB struct {
	transactionBlockID int64
	transactionMissing bool
}

func (d coreHistoryPrunedContractDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (d coreHistoryPrunedContractDB) Query(_ context.Context, _ string, _ ...interface{}) (pgx.Rows, error) {
	return coreHistoryPrunedContractRows{}, nil
}

func (d coreHistoryPrunedContractDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	if strings.Contains(query, "from core_transactions") && strings.Contains(query, "lower(tx_hash)") {
		if d.transactionMissing {
			return coreHistoryPrunedContractRow{err: pgx.ErrNoRows}
		}
		return coreHistoryPrunedContractRow{values: []interface{}{
			int64(1),
			d.transactionBlockID,
			int32(0),
			"0xold",
			[]byte("not used before block lookup"),
			pgtype.Timestamp{Time: time.Unix(0, 0), Valid: true},
		}}
	}
	return coreHistoryPrunedContractRow{err: pgx.ErrNoRows}
}

type coreHistoryPrunedContractRow struct {
	values []interface{}
	err    error
}

func (r coreHistoryPrunedContractRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *int64:
			*d = r.values[i].(int64)
		case *int32:
			*d = r.values[i].(int32)
		case *string:
			*d = r.values[i].(string)
		case *[]byte:
			*d = r.values[i].([]byte)
		case *pgtype.Timestamp:
			*d = r.values[i].(pgtype.Timestamp)
		default:
			panic("unexpected scan destination")
		}
	}
	return nil
}

type coreHistoryPrunedContractRows struct{}

func (coreHistoryPrunedContractRows) Close() {}

func (coreHistoryPrunedContractRows) Err() error {
	return nil
}

func (coreHistoryPrunedContractRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (coreHistoryPrunedContractRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (coreHistoryPrunedContractRows) Next() bool {
	return false
}

func (coreHistoryPrunedContractRows) Scan(...interface{}) error {
	return nil
}

func (coreHistoryPrunedContractRows) Values() ([]interface{}, error) {
	return nil, nil
}

func (coreHistoryPrunedContractRows) RawValues() [][]byte {
	return nil
}

func (coreHistoryPrunedContractRows) Conn() *pgx.Conn {
	return nil
}
