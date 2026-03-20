package main

import (
	"context"
	"fmt"
	"time"

	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"go.uber.org/zap"
)

func (r *Replayer) replaySocial(ctx context.Context) error {
	if err := r.replayFollows(ctx); err != nil {
		return fmt.Errorf("follows: %w", err)
	}
	if err := r.replaySaves(ctx); err != nil {
		return fmt.Errorf("saves: %w", err)
	}
	if err := r.replayReposts(ctx); err != nil {
		return fmt.Errorf("reposts: %w", err)
	}
	return nil
}

// --- Follows ---

func (r *Replayer) replayFollows(ctx context.Context) error {
	const countQ = `SELECT count(*) FROM follows WHERE is_current = true AND is_delete = false`
	const selectQ = `
		SELECT follower_user_id, followee_user_id
		FROM follows
		WHERE is_current = true AND is_delete = false
		ORDER BY follower_user_id, followee_user_id
		LIMIT $1 OFFSET $2`

	var total int64
	if err := r.srcDB.QueryRow(ctx, countQ).Scan(&total); err != nil {
		return fmt.Errorf("count: %w", err)
	}
	r.logger.Info("replaying follows", zap.Int64("total", total))

	type follow struct{ follower, followee int64 }
	return r.replayTable(ctx, "follows", total, selectQ,
		func(scan func(...any) error) (any, error) {
			var f follow
			return f, scan(&f.follower, &f.followee)
		},
		func(ctx context.Context, item any) error {
			f := item.(follow)
			return r.submitManageEntity(ctx, &corev1.ManageEntityLegacy{
				UserId:     f.follower,
				EntityType: "User",
				EntityId:   f.followee,
				Action:     "Follow",
				Metadata:   "",
			})
		},
	)
}

// --- Saves ---

func (r *Replayer) replaySaves(ctx context.Context) error {
	const countQ = `SELECT count(*) FROM saves WHERE is_current = true AND is_delete = false`
	const selectQ = `
		SELECT user_id, save_item_id, save_type
		FROM saves
		WHERE is_current = true AND is_delete = false
		ORDER BY user_id, save_item_id
		LIMIT $1 OFFSET $2`

	var total int64
	if err := r.srcDB.QueryRow(ctx, countQ).Scan(&total); err != nil {
		return fmt.Errorf("count: %w", err)
	}
	r.logger.Info("replaying saves", zap.Int64("total", total))

	type save struct {
		userID, itemID int64
		saveType       string
	}
	return r.replayTable(ctx, "saves", total, selectQ,
		func(scan func(...any) error) (any, error) {
			var s save
			return s, scan(&s.userID, &s.itemID, &s.saveType)
		},
		func(ctx context.Context, item any) error {
			s := item.(save)
			return r.submitManageEntity(ctx, &corev1.ManageEntityLegacy{
				UserId:     s.userID,
				EntityType: saveRepostEntityType(s.saveType),
				EntityId:   s.itemID,
				Action:     "Save",
				Metadata:   "",
			})
		},
	)
}

// --- Reposts ---

func (r *Replayer) replayReposts(ctx context.Context) error {
	const countQ = `SELECT count(*) FROM reposts WHERE is_current = true AND is_delete = false`
	const selectQ = `
		SELECT user_id, repost_item_id, repost_type
		FROM reposts
		WHERE is_current = true AND is_delete = false
		ORDER BY user_id, repost_item_id
		LIMIT $1 OFFSET $2`

	var total int64
	if err := r.srcDB.QueryRow(ctx, countQ).Scan(&total); err != nil {
		return fmt.Errorf("count: %w", err)
	}
	r.logger.Info("replaying reposts", zap.Int64("total", total))

	type repost struct {
		userID, itemID int64
		repostType     string
	}
	return r.replayTable(ctx, "reposts", total, selectQ,
		func(scan func(...any) error) (any, error) {
			var rp repost
			return rp, scan(&rp.userID, &rp.itemID, &rp.repostType)
		},
		func(ctx context.Context, item any) error {
			rp := item.(repost)
			return r.submitManageEntity(ctx, &corev1.ManageEntityLegacy{
				UserId:     rp.userID,
				EntityType: saveRepostEntityType(rp.repostType),
				EntityId:   rp.itemID,
				Action:     "Repost",
				Metadata:   "",
			})
		},
	)
}

// saveRepostEntityType maps save_type / repost_type strings to DP entity type names.
func saveRepostEntityType(t string) string {
	switch t {
	case "track":
		return "Track"
	case "playlist", "album":
		return "Playlist"
	default:
		return "Track"
	}
}

// replayTable is a generic paginated table replayer.
// scanFn scans a row into an item; submitFn submits that item to the chain.
func (r *Replayer) replayTable(
	ctx context.Context,
	entityType string,
	total int64,
	selectQ string,
	scanFn func(scan func(...any) error) (any, error),
	submitFn func(ctx context.Context, item any) error,
) error {
	sem := make(chan struct{}, r.cfg.Concurrency)
	batchStart := time.Now()
	var processed int64

	for offset := int64(0); offset < total; offset += int64(r.cfg.BatchSize) {
		if ctx.Err() != nil {
			break
		}

		rows, err := r.srcDB.Query(ctx, selectQ, r.cfg.BatchSize, offset)
		if err != nil {
			return fmt.Errorf("query at offset %d: %w", offset, err)
		}

		// Collect all rows before dispatching goroutines to avoid shared cursor.
		var items []any
		for rows.Next() {
			item, err := scanFn(rows.Scan)
			if err != nil {
				rows.Close()
				return fmt.Errorf("scan: %w", err)
			}
			items = append(items, item)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("rows error: %w", err)
		}

		for _, item := range items {
			if ctx.Err() != nil {
				break
			}
			item := item

			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()
				if err := submitFn(ctx, item); err != nil {
					if ctx.Err() == nil {
						r.logger.Warn("tx error",
							zap.String("type", entityType),
							zap.Error(err),
						)
						r.stats[entityType].Errors++
					}
				} else {
					r.stats[entityType].Submitted++
				}
			}()

			processed++
		}

		if processed%100000 == 0 && processed > 0 {
			r.logProgress(entityType, processed, total, batchStart)
		}
	}

	// Drain semaphore.
	for i := 0; i < r.cfg.Concurrency; i++ {
		sem <- struct{}{}
	}

	r.logProgress(entityType, processed, total, batchStart)
	return nil
}
