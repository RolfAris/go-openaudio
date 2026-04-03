package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/OpenAudio/go-openaudio/pkg/mediorum/cidutil"
	"go.uber.org/zap"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/labstack/echo/v4"
	"gocloud.dev/gcerrors"
	"golang.org/x/exp/slices"
	"gorm.io/gorm"
)

const (
	abortContextCanceled      = "CONTEXT_CANCELED"
	defaultRepairCleanupEvery = 4
	defaultQmCidsCleanupEvery = 1
)

// seenKeyResult stores the outcome of a previous Attributes check for the same
// key within one repair cycle, allowing duplicate checks to be skipped.
type seenKeyResult struct {
	alreadyHave bool
	size        int64
}

type RepairTracker struct {
	StartedAt         time.Time `gorm:"primaryKey;not null"`
	UpdatedAt         time.Time `gorm:"not null"`
	FinishedAt        time.Time
	CleanupMode       bool                     `gorm:"not null"`
	QmCidsCleanupMode bool                     `gorm:"not null;default:false"`
	CursorI           int                      `gorm:"not null"`
	CursorUploadID    string                   `gorm:"not null"`
	CursorPreviewCID  string                   ``
	CursorQmCID       string                   `gorm:"not null"`
	Counters          map[string]int           `gorm:"not null;serializer:json"`
	ContentSize       int64                    `gorm:"not null"`
	Duration          time.Duration            `gorm:"not null"`
	AbortedReason     string                   `gorm:"not null"`
	SeenKeys          map[string]seenKeyResult `gorm:"-" json:"-"`
}

func (ss *MediorumServer) startRepairer(ctx context.Context) error {
	logger := ss.logger.With(zap.String("task", "repair"))

	if !ss.Config.RepairEnabled {
		logger.Info("repair is disabled via OPENAUDIO_REPAIR_ENABLED=false")
		<-ctx.Done()
		return ctx.Err()
	}

	repairInterval := ss.Config.RepairInterval
	repairCleanupEvery := ss.Config.RepairCleanupEvery
	if repairCleanupEvery <= 0 {
		repairCleanupEvery = defaultRepairCleanupEvery
	}
	repairQmCidsCleanupEvery := ss.Config.RepairQmCidsCleanupEvery
	if repairQmCidsCleanupEvery < 0 {
		repairQmCidsCleanupEvery = defaultQmCidsCleanupEvery
	}
	logger.Info(
		"repair configured",
		zap.Duration("interval", repairInterval),
		zap.Int("cleanupEvery", repairCleanupEvery),
		zap.Int("qmCidsCleanupEvery", repairQmCidsCleanupEvery),
		zap.Bool("qmCidsUseListIndex", ss.Config.RepairQmCidsUseListIndex),
		zap.Int("qmCidsListIndexShadowCompareEvery", ss.Config.RepairQmCidsListIndexShadowCompareEvery),
		zap.Bool("qmCidsListIndexDisableOnMismatch", ss.Config.RepairQmCidsListIndexDisableOnMismatch),
	)

	// wait a minute on startup to determine healthy peers
	ticker := time.NewTicker(1 * time.Minute)
	for {
		select {
		case <-ticker.C:
			ticker.Reset(repairInterval)

			// pick up where we left off from the last repair.go run, including if the server restarted in the middle of a run
			tracker := RepairTracker{
				StartedAt:   time.Now(),
				CleanupMode: true,
				CursorI:     1,
				Counters:    map[string]int{},
			}
			var lastRun RepairTracker
			if err := ss.crud.DB.Order("started_at desc").First(&lastRun).Error; err == nil {
				if lastRun.FinishedAt.IsZero() {
					// resume previously interrupted job
					tracker = lastRun
				} else {
					// run the next job
					tracker.CursorI, tracker.CleanupMode = nextRepairCursor(lastRun.CursorI, repairCleanupEvery)
				}
			} else {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					logger.Error("failed to get last repair.go run", zap.Error(err))
				}
			}
			tracker.QmCidsCleanupMode = ss.qmCidsCleanupModeForRun(&tracker)
			logger := logger.With(
				zap.Int("run", tracker.CursorI),
				zap.Bool("cleanupMode", tracker.CleanupMode),
				zap.Bool("qmCidsCleanupMode", tracker.QmCidsCleanupMode),
			)

			saveTracker := func() {
				tracker.UpdatedAt = time.Now()
				if err := ss.crud.DB.Save(tracker).Error; err != nil {
					logger.Error("failed to save repair tracker", zap.Error(err))
				}
			}

			healthyPeers := ss.findHealthyPeers(time.Hour)
			if len(healthyPeers) < 1 {
				logger.Warn("not enough healthy peers to run repair",
					zap.Int("healthyPeers", len(healthyPeers)))
				tracker.AbortedReason = "NOT_ENOUGH_PEERS"
				tracker.FinishedAt = time.Now()
				saveTracker()
				// wait 1 minute before running again
				ticker.Reset(time.Minute * 1)
				continue
			}

			// check that disk has space
			if !ss.diskHasSpace() && !tracker.CleanupMode {
				logger.Warn("disk has <10GB remaining and is not in cleanup mode. skipping repair")
				tracker.AbortedReason = "DISK_FULL"
				tracker.FinishedAt = time.Now()
				saveTracker()
				// wait 1 minute before running again
				ticker.Reset(time.Minute * 1)
				continue
			}

			logger.Info("repair starting")
			err := ss.runRepair(ctx, &tracker)
			if err != nil && !errors.Is(err, context.Canceled) {
				tracker.FinishedAt = time.Now()
				logger.Error("repair failed", zap.Error(err), zap.Duration("took", tracker.Duration))
				tracker.AbortedReason = err.Error()
			} else if errors.Is(err, context.Canceled) {
				logger.Warn("repair interrupted", zap.Error(err), zap.Duration("took", tracker.Duration))
			} else {
				tracker.FinishedAt = time.Now()
				logger.Info("repair OK", zap.Duration("took", tracker.Duration))
				ss.lastSuccessfulRepair = tracker
				if tracker.CleanupMode {
					ss.lastSuccessfulCleanup = tracker
				}
			}
			saveTracker()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func nextRepairCursor(lastCursor int, cleanupEvery int) (int, bool) {
	if cleanupEvery <= 0 {
		cleanupEvery = defaultRepairCleanupEvery
	}
	cursor := lastCursor + 1
	if cursor > cleanupEvery {
		cursor = 1
	}
	return cursor, cursor == 1
}

func shouldRunQmCidsCleanup(cleanupOrdinal int64, cleanupEvery int) bool {
	if cleanupEvery < 0 {
		cleanupEvery = defaultQmCidsCleanupEvery
	}
	if cleanupEvery == 0 {
		return false
	}
	if cleanupOrdinal <= 0 {
		cleanupOrdinal = 1
	}
	return (cleanupOrdinal-1)%int64(cleanupEvery) == 0
}

func (ss *MediorumServer) qmCidsCleanupModeForRun(tracker *RepairTracker) bool {
	if tracker == nil || !tracker.CleanupMode {
		return false
	}
	cleanupEvery := ss.Config.RepairQmCidsCleanupEvery
	if cleanupEvery < 0 {
		cleanupEvery = defaultQmCidsCleanupEvery
	}
	var priorSuccessfulCleanups int64
	if err := ss.crud.DB.Model(&RepairTracker{}).
		Where("cleanup_mode = true and started_at < ? and finished_at is not null and finished_at != ? and aborted_reason = ?", tracker.StartedAt, time.Time{}, "").
		Count(&priorSuccessfulCleanups).Error; err != nil {
		ss.logger.Warn("failed to count prior successful cleanup runs for qm_cids cleanup cadence", zap.Error(err))
		return shouldRunQmCidsCleanup(1, cleanupEvery)
	}
	return shouldRunQmCidsCleanup(priorSuccessfulCleanups+1, cleanupEvery)
}

func (ss *MediorumServer) bucketPresenceForRepair(ctx context.Context, key string) (bool, int64, time.Time, error) {
	attrs, err := ss.bucket.Attributes(ctx, key)
	if err != nil {
		if gcerrors.Code(err) == gcerrors.NotFound || strings.Contains(err.Error(), "notFound") {
			return false, 0, time.Time{}, nil
		}
		return false, 0, time.Time{}, err
	}
	return true, attrs.Size, attrs.ModTime, nil
}

func (ss *MediorumServer) runRepair(ctx context.Context, tracker *RepairTracker) error {
	saveTracker := func() {
		tracker.UpdatedAt = time.Now()
		if err := ss.crud.DB.Save(tracker).Error; err != nil {
			ss.logger.Error("failed to save tracker", zap.Error(err))
		}
	}

	// scroll uploads and repair CIDs
	// (later this can clean up "derivative" images if we make image resizing dynamic)
	for {
		// abort if context is canceled
		if ctx.Err() != nil {
			tracker.AbortedReason = abortContextCanceled
			saveTracker()
			return ctx.Err()
		}

		// abort if disk is filling up
		if !ss.diskHasSpace() && !tracker.CleanupMode {
			tracker.AbortedReason = "DISK_FULL"
			saveTracker()
			break
		}

		startIter := time.Now()

		var uploads []Upload
		if err := ss.crud.DB.Where("id > ?", tracker.CursorUploadID).Order("id DESC").Limit(1000).Find(&uploads).Error; err != nil {
			return err
		}
		if len(uploads) == 0 {
			break
		}
		for _, u := range uploads {
			if ctx.Err() != nil {
				tracker.AbortedReason = abortContextCanceled
				break
			}

			tracker.CursorUploadID = u.ID
			ss.repairCid(ctx, u.OrigFileCID, u.PlacementHosts, tracker, repairSourceUploadOrig)
			// images are resized dynamically
			// so only consider audio TranscodeResults for repair
			if u.Template != JobTemplateAudio {
				continue
			}
			for _, cid := range u.TranscodeResults {
				ss.repairCid(ctx, cid, u.PlacementHosts, tracker, repairSourceUploadTranscode)
			}
		}

		tracker.Duration += time.Since(startIter)
		saveTracker()
	}

	// scroll audio_previews for repair
	for {
		// abort if context is canceled
		if ctx.Err() != nil {
			tracker.AbortedReason = abortContextCanceled
			saveTracker()
			return ctx.Err()
		}

		// abort if disk is filling up
		if !ss.diskHasSpace() && !tracker.CleanupMode {
			tracker.AbortedReason = "DISK_FULL"
			saveTracker()
			break
		}

		startIter := time.Now()

		var previews []AudioPreview
		if err := ss.crud.DB.Where("cid > ?", tracker.CursorPreviewCID).Order("cid").Limit(1000).Find(&previews).Error; err != nil {
			return err
		}
		if len(previews) == 0 {
			break
		}
		for _, u := range previews {
			if ctx.Err() != nil {
				tracker.AbortedReason = abortContextCanceled
				break
			}

			tracker.CursorPreviewCID = u.CID
			ss.repairCid(ctx, u.CID, nil, tracker, repairSourceAudioPreview)
		}

		tracker.Duration += time.Since(startIter)
		saveTracker()
	}

	qmCidsCleanupMode := tracker.CleanupMode && tracker.QmCidsCleanupMode
	var qmCidsPresenceIndex *repairPresenceIndex
	if qmCidsCleanupMode && ss.Config.RepairQmCidsUseListIndex {
		indexStartedAt := time.Now()
		index, err := ss.buildRepairPresenceIndex(ctx)
		if err != nil {
			ss.logger.Warn("failed to build qm_cids repair presence index; falling back to per-key bucket attributes", zap.Error(err))
			tracker.Counters["qm_cids_list_index_build_fail"]++
			tracker.Counters["qm_cids_list_index_build_duration_ms"] = int(time.Since(indexStartedAt).Milliseconds())
		} else {
			qmCidsPresenceIndex = index
			tracker.Counters["qm_cids_list_index_build_success"]++
			tracker.Counters["qm_cids_list_index_entries"] = index.Len()
			tracker.Counters["qm_cids_list_index_build_duration_ms"] = int(time.Since(indexStartedAt).Milliseconds())
			tracker.Duration += time.Since(indexStartedAt)
		}
	}

	// scroll older qm_cids table and repair
	for {
		// abort if context is canceled
		if ctx.Err() != nil {
			tracker.AbortedReason = abortContextCanceled
			saveTracker()
			return ctx.Err()
		}

		// abort if disk is filling up
		if !ss.diskHasSpace() && !tracker.CleanupMode {
			tracker.AbortedReason = "DISK_FULL"
			saveTracker()
			break
		}

		startIter := time.Now()

		var cidBatch []string
		err := pgxscan.Select(ctx, ss.pgPool, &cidBatch,
			`select key
			 from qm_cids
			 where key > $1
			 order by key
			 limit 1000`, tracker.CursorQmCID)

		if err != nil {
			return err
		}
		if len(cidBatch) == 0 {
			break
		}
		for _, cid := range cidBatch {
			if ctx.Err() != nil {
				tracker.AbortedReason = abortContextCanceled
				break
			}

			tracker.CursorQmCID = cid
			ss.repairCidWithPresenceIndex(ctx, cid, nil, tracker, repairSourceQmCID, qmCidsCleanupMode, qmCidsPresenceIndex)
		}

		tracker.Duration += time.Since(startIter)
		saveTracker()
	}

	return ctx.Err()
}

func (ss *MediorumServer) repairCid(ctx context.Context, cid string, placementHosts []string, tracker *RepairTracker, source string) error {
	if cid == "" {
		return nil
	}

	return ss.repairCidWithCleanupMode(ctx, cid, placementHosts, tracker, source, tracker.CleanupMode)
}

func (ss *MediorumServer) repairCidWithCleanupMode(
	ctx context.Context,
	cid string,
	placementHosts []string,
	tracker *RepairTracker,
	source string,
	cleanupMode bool,
) error {
	return ss.repairCidWithPresenceIndex(ctx, cid, placementHosts, tracker, source, cleanupMode, nil)
}

func (ss *MediorumServer) repairCidWithPresenceIndex(
	ctx context.Context,
	cid string,
	placementHosts []string,
	tracker *RepairTracker,
	source string,
	cleanupMode bool,
	presenceIndex *repairPresenceIndex,
) error {
	logger := ss.logger.With(zap.String("task", "repair"), zap.String("cid", cid), zap.Bool("cleanup", cleanupMode))
	ss.repairSourceEvidence.recordCall(source)

	preferredHosts, isMine := ss.rendezvousAllHosts(cid)

	// if placementHosts is specified
	isPlaced := len(placementHosts) > 0
	if isPlaced {
		// we're not a preferred host
		if !slices.Contains(placementHosts, ss.Config.Self.Host) {
			return nil
		}

		// we are a preffered host
		preferredHosts = placementHosts
		isMine = true
	}

	// fast path: do zero bucket ops if we know we don't care about this cid
	if !cleanupMode && !isMine {
		ss.repairSourceEvidence.recordFastSkipNotMine(source)
		return nil
	}

	tracker.Counters["total_checked"]++

	myRank := slices.Index(preferredHosts, ss.Config.Self.Host)

	key := cidutil.ShardCID(cid)

	// Per-cycle dedupe: repair iterates uploads, audio_previews, and qm_cids,
	// and the same CID can appear across those tables. Skip the duplicate
	// Attributes check when we already resolved this key earlier in the cycle.
	if tracker.SeenKeys == nil {
		tracker.SeenKeys = map[string]seenKeyResult{}
	}
	if prev, seen := tracker.SeenKeys[key]; seen {
		ss.repairSourceEvidence.recordCycle(source, true)
		tracker.Counters["repair_deduped"]++
		if prev.alreadyHave {
			tracker.Counters["already_have"]++
			tracker.ContentSize += prev.size
		}
		return nil
	}
	ss.repairSourceEvidence.recordCycle(source, false)
	alreadyHave := false
	blobSize := int64(0)
	blobModTime := time.Time{}
	var err error
	resolvedFromShadowCompare := false
	if presenceIndex != nil {
		if presenceIndex.ShouldUsePerKeyAttrsFallback() {
			tracker.Counters["qm_cids_list_index_fallback_lookup"]++
			ss.repairSourceEvidence.recordListIndexFallback(source)
		} else {
			tracker.Counters["qm_cids_list_index_lookup"]++
			if entry, ok := presenceIndex.Lookup(key); ok {
				alreadyHave = true
				blobSize = entry.Size
				blobModTime = entry.ModTime
				tracker.Counters["qm_cids_list_index_present"]++
			} else {
				tracker.Counters["qm_cids_list_index_missing"]++
			}

			if presenceIndex.ShouldShadowCompare(key) {
				tracker.Counters["qm_cids_list_index_shadow_compare_total"]++
				ss.repairSourceEvidence.recordShadowAttrCall(source)

				shadowAlreadyHave, shadowBlobSize, shadowBlobModTime, err := ss.bucketPresenceForRepair(ctx, key)
				if err != nil {
					tracker.Counters["read_attrs_fail"]++
					tracker.Counters["qm_cids_list_index_shadow_compare_fail"]++
					logger.Error("shadow compare exist check failed", zap.Error(err))
				} else {
					shadowMismatch := shadowAlreadyHave != alreadyHave
					if !shadowMismatch && shadowAlreadyHave {
						shadowMismatch = shadowBlobSize != blobSize || !repairPresenceIndexModTimesEquivalent(shadowBlobModTime, blobModTime)
					}
					resolvedFromShadowCompare = true
					if shadowMismatch {
						tracker.Counters["qm_cids_list_index_shadow_mismatch"]++
						ss.repairSourceEvidence.recordListIndexShadowMismatch(source)
						alreadyHave = shadowAlreadyHave
						blobSize = shadowBlobSize
						blobModTime = shadowBlobModTime
						if presenceIndex.disableOnShadowMismatch {
							presenceIndex.EnablePerKeyAttrsFallback()
							tracker.Counters["qm_cids_list_index_fallback_triggered"]++
							ss.repairSourceEvidence.recordListIndexFallback(source)
						}
					} else {
						tracker.Counters["qm_cids_list_index_shadow_match"]++
					}
				}
			}
		}
	}
	if (presenceIndex == nil || presenceIndex.ShouldUsePerKeyAttrsFallback()) && !resolvedFromShadowCompare {
		ss.repairSourceEvidence.recordAttrCall(source)
		alreadyHave, blobSize, blobModTime, err = ss.bucketPresenceForRepair(ctx, key)
		if err != nil {
			tracker.Counters["read_attrs_fail"]++
			logger.Error("exist check failed", zap.Error(err))
			alreadyHave = false
			blobSize = 0
			blobModTime = time.Time{}
		}
	} else {
		// already populated by the presence index path
	}

	// Store result for future duplicate checks within this cycle.
	tracker.SeenKeys[key] = seenKeyResult{alreadyHave: alreadyHave, size: blobSize}

	// in cleanup mode do some extra checks:
	// - validate CID, delete if invalid (doesn't apply to Qm keys because their hash is not the CID)
	if cleanupMode && alreadyHave && !cidutil.IsLegacyCID(cid) {
		if r, errRead := ss.bucket.NewReader(ctx, key, nil); errRead == nil {
			errVal := cidutil.ValidateCID(cid, r)
			errClose := r.Close()
			if errVal != nil {
				tracker.Counters["delete_invalid_needed"]++
				logger.Error("deleting invalid CID", zap.Error(errVal))
				if errDel := ss.bucket.Delete(ctx, key); errDel == nil {
					tracker.Counters["delete_invalid_success"]++
					tracker.SeenKeys[key] = seenKeyResult{alreadyHave: false}
				} else {
					tracker.Counters["delete_invalid_fail"]++
					logger.Error("failed to delete invalid CID", zap.Error(errDel))
				}
				return nil
			}

			if errClose != nil {
				logger.Error("failed to close blob reader", zap.Error(errClose))
			}
		} else {
			tracker.Counters["read_blob_fail"]++
			logger.Error("failed to read blob", zap.Error(errRead))
			return errRead
		}
	}

	// delete derived image variants since they'll be dynamically resized
	if strings.HasSuffix(cid, ".jpg") && !strings.HasSuffix(cid, "original.jpg") {
		if cleanupMode && alreadyHave {
			err := ss.dropFromMyBucket(cid)
			if err != nil {
				logger.Error("delete_resized_image_failed", zap.Error(err))
				tracker.Counters["delete_resized_image_failed"]++
			} else {
				tracker.Counters["delete_resized_image_ok"]++
			}
		}
		return nil
	}

	if alreadyHave {
		tracker.Counters["already_have"]++
		tracker.ContentSize += blobSize
	}

	// get blobs that I should have (regardless of health of other nodes)
	if isMine && !alreadyHave && ss.diskHasSpace() {
		ss.repairSourceEvidence.recordPullMineNeeded(source)
		tracker.Counters["pull_mine_needed"]++

		success := false
		// loop preferredHosts (not preferredHealthyHosts) because pullFileFromHost can still give us a file even if we thought the host was unhealthy
		for _, host := range preferredHosts {
			if host == ss.Config.Self.Host {
				continue
			}
			err := ss.pullFileFromHost(ctx, host, cid)
			if err != nil {
				tracker.Counters["pull_mine_fail"]++
				logger.Error("pull failed (blob I should have)", zap.Error(err), zap.String("host", host))
			} else {
				tracker.Counters["pull_mine_success"]++
				logger.Debug("pull OK (blob I should have)", zap.String("host", host))
				success = true

				pulledAttrs, errAttrs := ss.bucket.Attributes(ctx, key)
				if errAttrs != nil {
					tracker.ContentSize += pulledAttrs.Size
				}
				return nil
			}
		}
		if !success {
			logger.Warn("failed to pull from any host", zap.Strings("hosts", preferredHosts))
			return errors.New("failed to pull from any host")
		}
	}

	// delete over-replicated blobs:
	// check all healthy nodes ahead of me in the preferred order to ensure they have it.
	// if R+1 healthy nodes in front of me have it, I can safely delete.
	// don't delete if we replicated the blob within the past week
	wasReplicatedThisWeek := blobModTime.After(time.Now().Add(-24 * 7 * time.Hour))

	// by default retain blob if our rank < ReplicationFactor+2
	// but nodes with more free disk space will use a higher threshold
	// to accomidate "spill over" from nodes that might be full or down.
	diskPercentFree := float64(ss.mediorumPathFree) / float64(ss.mediorumPathSize)
	rankThreshold := ss.Config.ReplicationFactor + 2
	if !ss.diskHasSpace() {
		rankThreshold = ss.Config.ReplicationFactor
	} else if diskPercentFree > 0.4 {
		rankThreshold = ss.Config.ReplicationFactor * 3
	} else if diskPercentFree > 0.2 {
		rankThreshold = ss.Config.ReplicationFactor * 2
	}

	if !isPlaced && !ss.Config.StoreAll && tracker.CleanupMode && alreadyHave && myRank > rankThreshold && !wasReplicatedThisWeek {
		// if i'm the first node that over-replicated, keep the file for a week as a buffer since a node ahead of me in the preferred order will likely be down temporarily at some point
		tracker.Counters["delete_over_replicated_needed"]++
		err := ss.dropFromMyBucket(cid)
		if err != nil {
			tracker.Counters["delete_over_replicated_fail"]++
			logger.Error("delete failed", zap.Error(err))
			return err
		} else {
			tracker.Counters["delete_over_replicated_success"]++
			logger.Debug("delete OK")
			tracker.ContentSize -= blobSize
			return nil
		}
	}

	return nil
}

func (ss *MediorumServer) serveRepairLog(c echo.Context) error {
	limitStr := c.QueryParam("limit")
	if limitStr == "" {
		limitStr = "1000"
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return c.String(http.StatusBadRequest, "Invalid limit value")
	}

	if limit > 1000 {
		limit = 1000
	}

	var logs []RepairTracker
	if err := ss.crud.DB.Order("started_at desc").Limit(limit).Find(&logs).Error; err != nil {
		return c.String(http.StatusInternalServerError, "DB query failed")
	}

	return c.JSON(http.StatusOK, logs)
}
