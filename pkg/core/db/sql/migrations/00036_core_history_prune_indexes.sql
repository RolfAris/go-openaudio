-- +migrate Up notransaction
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_core_tx_stats_block_height
  ON core_tx_stats(block_height);

-- +migrate Down notransaction
DROP INDEX CONCURRENTLY IF EXISTS idx_core_tx_stats_block_height;
