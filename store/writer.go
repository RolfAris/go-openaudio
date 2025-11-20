package store

import (
	"github.com/cockroachdb/pebble"
)

// WriteBatch wraps a Pebble batch and provides safe, atomic write helpers.
// Use WriteBatch.StoreXXX() methods to modify state.
// Call Commit() at end-of-block, otherwise discard with DeferDiscard().
type WriteBatch struct {
	db    *pebble.DB
	batch *pebble.Batch
	done  bool
}

// Commit writes the batch atomically to Pebble.
func (wb *WriteBatch) Commit(sync bool) error {
	if wb.done {
		return nil
	}
	wb.done = true

	if sync {
		return wb.batch.Commit(pebble.Sync)
	}
	return wb.batch.Commit(pebble.NoSync)
}

// Discard drops changes if Commit wasn’t called.
func (wb *WriteBatch) Discard() {
	if wb.done {
		return
	}
	wb.done = true
	_ = wb.batch.Close()
}

// CommitOrDiscard calls Commit(sync=true) if not already committed.
func (wb *WriteBatch) CommitOrDiscard() error {
	if wb.done {
		return nil
	}
	err := wb.Commit(true)
	if err != nil {
		wb.Discard()
	}
	return err
}
