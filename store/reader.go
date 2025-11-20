package store

import (
	"github.com/cockroachdb/pebble"
)

// Reader provides safe read-only access to Pebble.
// It creates snapshots to ensure consistent reads.
type Reader struct {
	db *pebble.DB
}

// Get returns a copy of the value for key.
func (r *Reader) Get(key []byte) ([]byte, error) {
	val, closer, err := r.db.Get(key)
	if err != nil {
		if err == pebble.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	defer closer.Close()

	out := make([]byte, len(val))
	copy(out, val)
	return out, nil
}

// Iter iterates over a key range using a snapshot.
func (r *Reader) Iter(lower, upper []byte, fn func(key, val []byte) error) error {
	snap := r.db.NewSnapshot()
	defer snap.Close()

	iter, err := snap.NewIter(&pebble.IterOptions{
		LowerBound: lower,
		UpperBound: upper,
	})
	if err != nil {
		return err
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		k := append([]byte{}, iter.Key()...)
		v := append([]byte{}, iter.Value()...)

		if err := fn(k, v); err != nil {
			return err
		}
	}

	return iter.Error()
}
