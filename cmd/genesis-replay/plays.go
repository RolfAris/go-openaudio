package main

import (
	"context"
	"fmt"
	"time"

	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type sourcePlay struct {
	UserID    *string
	TrackID   string
	CreatedAt time.Time
	City      *string
	Region    *string
	Country   *string
}

// replayPlays replays individual play events as TrackPlay transactions.
// For large datasets, use --skip-plays and handle plays separately.
// Plays are batched into TrackPlays messages (up to playsPerBatch per tx).
func (r *Replayer) replayPlays(ctx context.Context) error {
	const playsPerBatch = 100

	const countQ = `SELECT count(*) FROM plays`
	const selectQ = `
		SELECT
			user_id::text, play_item_id::text, created_at,
			city, region, country
		FROM plays
		ORDER BY created_at, play_item_id
		LIMIT $1 OFFSET $2`

	var total int64
	if err := r.srcDB.QueryRow(ctx, countQ).Scan(&total); err != nil {
		return fmt.Errorf("count plays: %w", err)
	}
	r.logger.Info("replaying plays", zap.Int64("total", total))

	batchStart := time.Now()
	var processed int64

	// Outer pagination over the plays table.
	for offset := int64(0); offset < total; offset += int64(r.cfg.BatchSize) {
		if ctx.Err() != nil {
			break
		}

		rows, err := r.srcDB.Query(ctx, selectQ, r.cfg.BatchSize, offset)
		if err != nil {
			return fmt.Errorf("query plays at offset %d: %w", offset, err)
		}

		var plays []sourcePlay
		for rows.Next() {
			var p sourcePlay
			if err := rows.Scan(&p.UserID, &p.TrackID, &p.CreatedAt, &p.City, &p.Region, &p.Country); err != nil {
				rows.Close()
				return fmt.Errorf("scan play: %w", err)
			}
			plays = append(plays, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("plays rows error: %w", err)
		}

		// Chunk into TrackPlays messages.
		for i := 0; i < len(plays); i += playsPerBatch {
			if ctx.Err() != nil {
				break
			}

			end := i + playsPerBatch
			if end > len(plays) {
				end = len(plays)
			}
			chunk := plays[i:end]

			trackPlays := make([]*corev1.TrackPlay, 0, len(chunk))
			for _, p := range chunk {
				tp := &corev1.TrackPlay{
					TrackId:   p.TrackID,
					Timestamp: timestamppb.New(p.CreatedAt),
				}
				if p.UserID != nil {
					tp.UserId = *p.UserID
				}
				if p.City != nil {
					tp.City = *p.City
				}
				if p.Region != nil {
					tp.Region = *p.Region
				}
				if p.Country != nil {
					tp.Country = *p.Country
				}
				trackPlays = append(trackPlays, tp)
			}

			req := &corev1.ForwardTransactionRequest{
				Transaction: &corev1.SignedTransaction{
					RequestId: uuid.NewString(),
					Transaction: &corev1.SignedTransaction_Plays{
						Plays: &corev1.TrackPlays{
							Plays: trackPlays,
						},
					},
				},
			}

			if err := r.forwardWithRetry(ctx, req); err != nil {
				if ctx.Err() == nil {
					r.logger.Warn("plays tx error", zap.Int("chunk_size", len(chunk)), zap.Error(err))
					r.stats["plays"].Errors += int64(len(chunk))
				}
			} else {
				r.stats["plays"].Submitted += int64(len(chunk))
			}

			processed += int64(len(chunk))
		}

		if processed%1000000 == 0 && processed > 0 {
			r.logProgress("plays", processed, total, batchStart)
		}
	}

	r.logProgress("plays", processed, total, batchStart)
	return nil
}
