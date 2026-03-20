package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// trackMetadataWrapper is the CID-wrapped format the DP expects for Track CREATE/UPDATE.
type trackMetadataWrapper struct {
	CID  string             `json:"cid"`
	Data trackMetadataInner `json:"data"`
}

type trackMetadataInner struct {
	Title              string      `json:"title,omitempty"`
	OwnerID            int64       `json:"owner_id"`
	Duration           int         `json:"duration,omitempty"`
	Description        string      `json:"description,omitempty"`
	Genre              string      `json:"genre,omitempty"`
	Mood               string      `json:"mood,omitempty"`
	Tags               string      `json:"tags,omitempty"`
	TrackCID           string      `json:"track_cid,omitempty"`
	PreviewCID         string      `json:"preview_cid,omitempty"`
	CoverArt           string      `json:"cover_art,omitempty"`
	CoverArtSizes      string      `json:"cover_art_sizes,omitempty"`
	IsUnlisted         bool        `json:"is_unlisted,omitempty"`
	IsDownloadable     bool        `json:"is_downloadable,omitempty"`
	IsOriginalAvail    bool        `json:"is_original_available,omitempty"`
	ReleaseDate        string      `json:"release_date,omitempty"`
	License            string      `json:"license,omitempty"`
	ISRC               string      `json:"isrc,omitempty"`
	ISWC               string      `json:"iswc,omitempty"`
	BPM                *float64    `json:"bpm,omitempty"`
	MusicalKey         string      `json:"musical_key,omitempty"`
	RemixOf            interface{} `json:"remix_of,omitempty"`
	StemOf             interface{} `json:"stem_of,omitempty"`
	IsStreamGated      bool        `json:"is_stream_gated,omitempty"`
	StreamConditions   interface{} `json:"stream_conditions,omitempty"`
	IsDownloadGated    bool        `json:"is_download_gated,omitempty"`
	DownloadConditions interface{} `json:"download_conditions,omitempty"`
}

type sourceTrack struct {
	TrackID              int64
	OwnerID              int64
	Title                *string
	Description          *string
	Duration             *int
	Genre                *string
	Mood                 *string
	Tags                 *string
	MetadataMultihash    *string
	TrackSegments        []byte // JSONB
	CoverArt             *string
	CoverArtSizes        *string
	PreviewCID           *string
	IsUnlisted           bool
	IsDownloadable       bool
	IsOriginalAvailable  bool
	ReleaseDate          *string
	License              *string
	ISRC                 *string
	ISWC                 *string
	BPM                  *float64
	MusicalKey           *string
	RemixOf              []byte // JSONB
	StemOf               []byte // JSONB
	IsStreamGated        bool
	StreamConditions     []byte // JSONB
	IsDownloadGated      bool
	DownloadConditions   []byte // JSONB
}

func (r *Replayer) replayTracks(ctx context.Context) error {
	const countQ = `SELECT count(*) FROM tracks WHERE is_current = true AND is_delete = false AND is_available = true`
	const selectQ = `
		SELECT
			track_id, owner_id, title, description, duration, genre, mood, tags,
			metadata_multihash, track_segments,
			cover_art, cover_art_sizes, preview_cid,
			is_unlisted, is_downloadable, is_original_available,
			release_date::text, license, isrc, iswc, bpm, musical_key,
			remix_of, stem_of,
			is_stream_gated, stream_conditions,
			is_download_gated, download_conditions
		FROM tracks
		WHERE is_current = true AND is_delete = false AND is_available = true
		ORDER BY track_id
		LIMIT $1 OFFSET $2`

	var total int64
	if err := r.srcDB.QueryRow(ctx, countQ).Scan(&total); err != nil {
		return fmt.Errorf("count tracks: %w", err)
	}
	r.logger.Info("replaying tracks", zap.Int64("total", total))

	sem := make(chan struct{}, r.cfg.Concurrency)
	batchStart := time.Now()
	var processed int64

	for offset := int64(0); offset < total; offset += int64(r.cfg.BatchSize) {
		if ctx.Err() != nil {
			break
		}

		rows, err := r.srcDB.Query(ctx, selectQ, r.cfg.BatchSize, offset)
		if err != nil {
			return fmt.Errorf("query tracks at offset %d: %w", offset, err)
		}

		tracks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (sourceTrack, error) {
			var t sourceTrack
			err := row.Scan(
				&t.TrackID, &t.OwnerID, &t.Title, &t.Description, &t.Duration,
				&t.Genre, &t.Mood, &t.Tags,
				&t.MetadataMultihash, &t.TrackSegments,
				&t.CoverArt, &t.CoverArtSizes, &t.PreviewCID,
				&t.IsUnlisted, &t.IsDownloadable, &t.IsOriginalAvailable,
				&t.ReleaseDate, &t.License, &t.ISRC, &t.ISWC,
				&t.BPM, &t.MusicalKey,
				&t.RemixOf, &t.StemOf,
				&t.IsStreamGated, &t.StreamConditions,
				&t.IsDownloadGated, &t.DownloadConditions,
			)
			return t, err
		})
		if err != nil {
			return fmt.Errorf("scan tracks: %w", err)
		}

		for _, t := range tracks {
			if ctx.Err() != nil {
				break
			}
			t := t

			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()
				if err := r.submitTrack(ctx, t); err != nil {
					if ctx.Err() == nil {
						r.logger.Warn("track tx error", zap.Int64("track_id", t.TrackID), zap.Error(err))
						r.stats["tracks"].Errors++
					}
				} else {
					r.stats["tracks"].Submitted++
				}
			}()

			processed++
		}

		if processed%10000 == 0 {
			r.logProgress("tracks", processed, total, batchStart)
		}
	}

	for i := 0; i < r.cfg.Concurrency; i++ {
		sem <- struct{}{}
	}

	r.logProgress("tracks", processed, total, batchStart)
	return nil
}

func (r *Replayer) submitTrack(ctx context.Context, t sourceTrack) error {
	inner := trackMetadataInner{
		OwnerID:           t.OwnerID,
		IsUnlisted:        t.IsUnlisted,
		IsDownloadable:    t.IsDownloadable,
		IsOriginalAvail:   t.IsOriginalAvailable,
		IsStreamGated:     t.IsStreamGated,
		IsDownloadGated:   t.IsDownloadGated,
	}
	if t.Title != nil {
		inner.Title = *t.Title
	}
	if t.Description != nil {
		inner.Description = *t.Description
	}
	if t.Duration != nil {
		inner.Duration = *t.Duration
	}
	if t.Genre != nil {
		inner.Genre = *t.Genre
	}
	if t.Mood != nil {
		inner.Mood = *t.Mood
	}
	if t.Tags != nil {
		inner.Tags = *t.Tags
	}
	if t.CoverArt != nil {
		inner.CoverArt = *t.CoverArt
	}
	if t.CoverArtSizes != nil {
		inner.CoverArtSizes = *t.CoverArtSizes
	}
	if t.PreviewCID != nil {
		inner.PreviewCID = *t.PreviewCID
	}
	if t.ReleaseDate != nil {
		inner.ReleaseDate = *t.ReleaseDate
	}
	if t.License != nil {
		inner.License = *t.License
	}
	if t.ISRC != nil {
		inner.ISRC = *t.ISRC
	}
	if t.ISWC != nil {
		inner.ISWC = *t.ISWC
	}
	if t.BPM != nil {
		inner.BPM = t.BPM
	}
	if t.MusicalKey != nil {
		inner.MusicalKey = *t.MusicalKey
	}
	if len(t.RemixOf) > 0 {
		var v interface{}
		if err := json.Unmarshal(t.RemixOf, &v); err == nil {
			inner.RemixOf = v
		}
	}
	if len(t.StemOf) > 0 {
		var v interface{}
		if err := json.Unmarshal(t.StemOf, &v); err == nil {
			inner.StemOf = v
		}
	}
	if len(t.StreamConditions) > 0 {
		var v interface{}
		if err := json.Unmarshal(t.StreamConditions, &v); err == nil {
			inner.StreamConditions = v
		}
	}
	if len(t.DownloadConditions) > 0 {
		var v interface{}
		if err := json.Unmarshal(t.DownloadConditions, &v); err == nil {
			inner.DownloadConditions = v
		}
	}

	// Use metadata_multihash as the CID in the wrapper; fall back to a placeholder.
	cid := "genesis-import"
	if t.MetadataMultihash != nil && *t.MetadataMultihash != "" {
		cid = *t.MetadataMultihash
	}

	// track_cid: pull from track_segments if available, otherwise leave empty.
	// The DP will store it; streaming won't work without a real CID but the
	// metadata record will be correct.
	if len(t.TrackSegments) > 0 {
		var segs []struct {
			MultiHash string `json:"multihash"`
		}
		if err := json.Unmarshal(t.TrackSegments, &segs); err == nil && len(segs) > 0 {
			inner.TrackCID = segs[0].MultiHash
		}
	}

	wrapper := trackMetadataWrapper{CID: cid, Data: inner}
	metaJSON, err := json.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("marshal track metadata: %w", err)
	}

	return r.submitManageEntity(ctx, &corev1.ManageEntityLegacy{
		UserId:     t.OwnerID,
		EntityType: "Track",
		EntityId:   t.TrackID,
		Action:     "Create",
		Metadata:   string(metaJSON),
	})
}
