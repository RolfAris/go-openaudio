package server

import (
	"context"
	"hash/fnv"
	"io"
	"math"
	"time"
)

type repairPresenceIndexEntry struct {
	Size    int64
	ModTime time.Time
}

type repairPresenceIndex struct {
	entries                 map[string]repairPresenceIndexEntry
	shadowCompareEvery      int
	disableOnShadowMismatch bool
	fallbackToPerKeyAttrs   bool
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

func (idx *repairPresenceIndex) ShouldUsePerKeyAttrsFallback() bool {
	return idx != nil && idx.fallbackToPerKeyAttrs
}

func (idx *repairPresenceIndex) EnablePerKeyAttrsFallback() {
	if idx == nil {
		return
	}
	idx.fallbackToPerKeyAttrs = true
}

func (idx *repairPresenceIndex) ShouldShadowCompare(key string) bool {
	if idx == nil || idx.shadowCompareEvery <= 0 {
		return false
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(key))
	return int(hasher.Sum32()%uint32(idx.shadowCompareEvery)) == 0
}

func repairPresenceIndexModTimesEquivalent(listModTime, attrModTime time.Time) bool {
	if listModTime.Equal(attrModTime) {
		return true
	}
	if listModTime.IsZero() || attrModTime.IsZero() {
		return false
	}
	return math.Abs(listModTime.Sub(attrModTime).Seconds()) < 1
}

func (ss *MediorumServer) buildRepairPresenceIndex(ctx context.Context) (*repairPresenceIndex, error) {
	iter := ss.bucket.List(nil)
	index := &repairPresenceIndex{
		entries:                 map[string]repairPresenceIndexEntry{},
		shadowCompareEvery:      ss.Config.RepairQmCidsListIndexShadowCompareEvery,
		disableOnShadowMismatch: ss.Config.RepairQmCidsListIndexDisableOnMismatch,
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
