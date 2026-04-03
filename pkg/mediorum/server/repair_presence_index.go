package server

import (
	"context"
	"io"
	"time"
)

type repairPresenceIndexEntry struct {
	Size    int64
	ModTime time.Time
}

type repairPresenceIndex struct {
	entries map[string]repairPresenceIndexEntry
}

func (idx *repairPresenceIndex) Len() int {
	if idx == nil {
		return 0
	}
	return len(idx.entries)
}

func (idx *repairPresenceIndex) Lookup(key string) (repairPresenceIndexEntry, bool) {
	if idx == nil {
		return repairPresenceIndexEntry{}, false
	}
	entry, ok := idx.entries[key]
	return entry, ok
}

func (ss *MediorumServer) buildRepairPresenceIndex(ctx context.Context) (*repairPresenceIndex, error) {
	iter := ss.bucket.List(nil)
	index := &repairPresenceIndex{
		entries: map[string]repairPresenceIndexEntry{},
	}

	for {
		obj, err := iter.Next(ctx)
		if err != nil {
			if err == io.EOF {
				return index, nil
			}
			return nil, err
		}
		if obj == nil || obj.IsDir {
			continue
		}
		index.entries[obj.Key] = repairPresenceIndexEntry{
			Size:    obj.Size,
			ModTime: obj.ModTime,
		}
	}
}
