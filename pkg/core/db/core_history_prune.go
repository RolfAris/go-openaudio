package db

import (
	"context"
	"fmt"
)

type CoreHistoryPruneResult struct {
	RetainFloorHeight int64                         `json:"retain_floor_height"`
	BatchSize         int64                         `json:"batch_size"`
	DeletedRows       int64                         `json:"deleted_rows"`
	Complete          bool                          `json:"complete"`
	Tables            []CoreHistoryPruneTableResult `json:"tables"`
}

type CoreHistoryPruneTableResult struct {
	Name        string `json:"name"`
	DeletedRows int64  `json:"deleted_rows"`
}

type coreHistoryPruneTable struct {
	name         string
	heightColumn string
}

var coreHistoryPruneTables = []coreHistoryPruneTable{
	{name: "core_transactions", heightColumn: "block_id"},
	{name: "core_tx_stats", heightColumn: "block_height"},
	{name: "core_app_state", heightColumn: "block_height"},
	{name: "core_blocks", heightColumn: "height"},
}

func (q *Queries) PruneCoreHistory(ctx context.Context, retainFloorHeight, batchSize int64) (*CoreHistoryPruneResult, error) {
	result := &CoreHistoryPruneResult{
		RetainFloorHeight: retainFloorHeight,
		BatchSize:         batchSize,
		Complete:          true,
		Tables:            make([]CoreHistoryPruneTableResult, 0, len(coreHistoryPruneTables)),
	}
	if retainFloorHeight <= 1 || batchSize <= 0 {
		return result, nil
	}

	for _, table := range coreHistoryPruneTables {
		deletedRows, err := q.pruneCoreHistoryTable(ctx, table, retainFloorHeight, batchSize)
		if err != nil {
			return result, err
		}
		result.DeletedRows += deletedRows
		if deletedRows >= batchSize {
			result.Complete = false
		}
		result.Tables = append(result.Tables, CoreHistoryPruneTableResult{
			Name:        table.name,
			DeletedRows: deletedRows,
		})
	}

	return result, nil
}

func (q *Queries) pruneCoreHistoryTable(ctx context.Context, table coreHistoryPruneTable, retainFloorHeight, batchSize int64) (int64, error) {
	var deletedRows int64
	if err := q.db.QueryRow(ctx, coreHistoryPruneSQL(table), retainFloorHeight, batchSize).Scan(&deletedRows); err != nil {
		return 0, fmt.Errorf("prune %s: %w", table.name, err)
	}
	return deletedRows, nil
}

func coreHistoryPruneSQL(table coreHistoryPruneTable) string {
	return fmt.Sprintf(`
with deleted as (
	delete from %s
	where ctid in (
		select ctid
		from %s
		where %s < $1
		order by %s
		limit $2
	)
	returning 1
)
select count(*)::bigint from deleted
`, table.name, table.name, table.heightColumn, table.heightColumn)
}
