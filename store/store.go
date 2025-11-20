package store

import (
	"github.com/cockroachdb/pebble"
)

// Store is the root Pebble-backed storage engine for the OpenAudio node.
// It provides:
//   - NewBatch(): atomic write transactions (ABCI/consensus)
//   - Reader(): safe snapshot-based read operations (RPC/Connect)
//
// Only the consensus pipeline should call NewBatch().
type Store struct {
	db *pebble.DB
}

// NewStore opens the Pebble database at the given path.
func NewStore(path string, opts *pebble.Options) (*Store, error) {
	if opts == nil {
		opts = &pebble.Options{}
	}

	db, err := pebble.Open(path, opts)
	if err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close gracefully closes the underlying Pebble DB.
func (s *Store) Close() error {
	return s.db.Close()
}

// NewBatch returns a WriteBatch used for atomic multi-key writes.
// This is the ONLY way anything should write to Pebble.
// CometBFT pipeline (DeliverTx/EndBlock/Commit) uses this.
func (s *Store) NewBatch() *WriteBatch {
	return &WriteBatch{
		db:    s.db,
		batch: s.db.NewBatch(),
	}
}

// Reader returns a read-only access wrapper.
// RPC/Connect and internal read paths use this.
// Reader NEVER writes to Pebble.
func (s *Store) Reader() *Reader {
	return &Reader{db: s.db}
}
