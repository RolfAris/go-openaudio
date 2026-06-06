package db

import (
	"context"
	"math"

	"github.com/jackc/pgx/v5/pgtype"
)

type CoreHistoryStatus struct {
	RetainFloorHeight                   int64                    `json:"retain_floor_height"`
	Tables                              []CoreHistoryTableStatus `json:"tables"`
	TotalRelationBytes                  int64                    `json:"total_relation_bytes"`
	EstimatedBytesBelowRetainFloor      int64                    `json:"estimated_bytes_below_retain_floor"`
	EstimatedBytesBelowRetainFloorKnown bool                     `json:"estimated_bytes_below_retain_floor_known"`
}

type CoreHistoryTableStatus struct {
	Name                                string `json:"name"`
	HeightColumn                        string `json:"height_column"`
	HeightBoundsIndexed                 bool   `json:"height_bounds_indexed"`
	Exists                              bool   `json:"exists"`
	EstimatedRows                       int64  `json:"estimated_rows"`
	RelationBytes                       int64  `json:"relation_bytes"`
	MinHeight                           *int64 `json:"min_height,omitempty"`
	MaxHeight                           *int64 `json:"max_height,omitempty"`
	EstimatedRowsBelowRetainFloor       *int64 `json:"estimated_rows_below_retain_floor,omitempty"`
	EstimatedBytesBelowRetainFloor      *int64 `json:"estimated_bytes_below_retain_floor,omitempty"`
	BelowRetainFloorEstimateUnavailable string `json:"below_retain_floor_estimate_unavailable,omitempty"`
}

type coreHistoryTable struct {
	name                string
	heightColumn        string
	heightBoundsIndexed bool
	heightBoundsSQL     string
}

var coreHistoryTables = []coreHistoryTable{
	{
		name:                "core_blocks",
		heightColumn:        "height",
		heightBoundsIndexed: true,
		heightBoundsSQL:     `select min(height)::bigint, max(height)::bigint from core_blocks`,
	},
	{
		name:                "core_transactions",
		heightColumn:        "block_id",
		heightBoundsIndexed: true,
		heightBoundsSQL:     `select min(block_id)::bigint, max(block_id)::bigint from core_transactions`,
	},
	{
		name:                "core_app_state",
		heightColumn:        "block_height",
		heightBoundsIndexed: true,
		heightBoundsSQL:     `select min(block_height)::bigint, max(block_height)::bigint from core_app_state`,
	},
	{
		name:                "core_tx_stats",
		heightColumn:        "block_height",
		heightBoundsIndexed: false,
	},
}

const coreHistoryRelationStatsSQL = `
with rel as (
	select to_regclass($1) as oid
)
select
	rel.oid is not null,
	case when rel.oid is null then 0 else pg_total_relation_size(rel.oid) end,
	coalesce(greatest(c.reltuples, 0), 0)
from rel
left join pg_class c on c.oid = rel.oid
`

func (q *Queries) GetCoreHistoryStatus(ctx context.Context, retainFloorHeight int64) (*CoreHistoryStatus, error) {
	status := &CoreHistoryStatus{
		RetainFloorHeight: retainFloorHeight,
		Tables:            make([]CoreHistoryTableStatus, 0, len(coreHistoryTables)),
	}

	for _, table := range coreHistoryTables {
		tableStatus, err := q.getCoreHistoryTableStatus(ctx, table, retainFloorHeight)
		if err != nil {
			return nil, err
		}

		status.Tables = append(status.Tables, tableStatus)
		status.TotalRelationBytes += tableStatus.RelationBytes
		if tableStatus.EstimatedBytesBelowRetainFloor != nil {
			status.EstimatedBytesBelowRetainFloor += *tableStatus.EstimatedBytesBelowRetainFloor
			status.EstimatedBytesBelowRetainFloorKnown = true
		}
	}

	return status, nil
}

func (q *Queries) getCoreHistoryTableStatus(ctx context.Context, table coreHistoryTable, retainFloorHeight int64) (CoreHistoryTableStatus, error) {
	status := CoreHistoryTableStatus{
		Name:                table.name,
		HeightColumn:        table.heightColumn,
		HeightBoundsIndexed: table.heightBoundsIndexed,
	}

	var estimatedRows float64
	if err := q.db.QueryRow(ctx, coreHistoryRelationStatsSQL, table.name).Scan(&status.Exists, &status.RelationBytes, &estimatedRows); err != nil {
		return status, err
	}
	status.EstimatedRows = int64(math.Round(estimatedRows))

	if !status.Exists {
		status.BelowRetainFloorEstimateUnavailable = "table does not exist"
		return status, nil
	}

	if !table.heightBoundsIndexed {
		status.BelowRetainFloorEstimateUnavailable = "height bounds skipped because block_height is not indexed"
		return status, nil
	}

	minHeight, maxHeight, err := q.coreHistoryHeightBounds(ctx, table.heightBoundsSQL)
	if err != nil {
		return status, err
	}
	status.MinHeight = minHeight
	status.MaxHeight = maxHeight

	estimatedRowsBelow, estimatedBytesBelow := estimateCoreHistoryBelowRetainFloor(
		minHeight,
		maxHeight,
		status.EstimatedRows,
		status.RelationBytes,
		retainFloorHeight,
	)
	status.EstimatedRowsBelowRetainFloor = estimatedRowsBelow
	status.EstimatedBytesBelowRetainFloor = estimatedBytesBelow
	if estimatedRowsBelow == nil {
		status.BelowRetainFloorEstimateUnavailable = "height bounds unavailable"
	}

	return status, nil
}

func (q *Queries) coreHistoryHeightBounds(ctx context.Context, query string) (*int64, *int64, error) {
	var minHeight pgtype.Int8
	var maxHeight pgtype.Int8
	if err := q.db.QueryRow(ctx, query).Scan(&minHeight, &maxHeight); err != nil {
		return nil, nil, err
	}

	if !minHeight.Valid || !maxHeight.Valid {
		return nil, nil, nil
	}

	return &minHeight.Int64, &maxHeight.Int64, nil
}

func estimateCoreHistoryBelowRetainFloor(minHeight, maxHeight *int64, estimatedRows, relationBytes, retainFloorHeight int64) (*int64, *int64) {
	if minHeight == nil || maxHeight == nil || *maxHeight < *minHeight {
		return nil, nil
	}

	span := *maxHeight - *minHeight + 1
	belowSpan := retainFloorHeight - *minHeight
	if belowSpan < 0 {
		belowSpan = 0
	}
	if belowSpan > span {
		belowSpan = span
	}

	fraction := float64(belowSpan) / float64(span)
	rows := int64(math.Round(float64(estimatedRows) * fraction))
	bytes := int64(math.Round(float64(relationBytes) * fraction))
	return &rows, &bytes
}
