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

type playlistMetadataWrapper struct {
	CID  string                `json:"cid"`
	Data playlistMetadataInner `json:"data"`
}

type playlistMetadataInner struct {
	PlaylistName              string      `json:"playlist_name"`
	Description               string      `json:"description,omitempty"`
	IsAlbum                   bool        `json:"is_album,omitempty"`
	IsPrivate                 bool        `json:"is_private,omitempty"`
	PlaylistImageSizesHash    string      `json:"playlist_image_sizes_multihash,omitempty"`
	PlaylistContents          interface{} `json:"playlist_contents,omitempty"`
	ReleaseDate               string      `json:"release_date,omitempty"`
	IsStreamGated             bool        `json:"is_stream_gated,omitempty"`
	StreamConditions          interface{} `json:"stream_conditions,omitempty"`
	UPC                       string      `json:"upc,omitempty"`
}

type sourcePlaylist struct {
	PlaylistID          int64
	PlaylistOwnerID     int64
	PlaylistName        *string
	Description         *string
	IsAlbum             bool
	IsPrivate           bool
	MetadataMultihash   *string
	ImageSizesMultihash *string
	PlaylistContents    []byte // JSONB
	ReleaseDate         *string
	IsStreamGated       bool
	StreamConditions    []byte // JSONB
}

func (r *Replayer) replayPlaylists(ctx context.Context) error {
	const countQ = `SELECT count(*) FROM playlists WHERE is_current = true AND is_delete = false`
	const selectQ = `
		SELECT
			playlist_id, playlist_owner_id, playlist_name, description,
			is_album, is_private,
			metadata_multihash, playlist_image_sizes_multihash, playlist_contents,
			release_date::text, is_stream_gated, stream_conditions
		FROM playlists
		WHERE is_current = true AND is_delete = false
		ORDER BY playlist_id
		LIMIT $1 OFFSET $2`

	var total int64
	if err := r.srcDB.QueryRow(ctx, countQ).Scan(&total); err != nil {
		return fmt.Errorf("count playlists: %w", err)
	}
	r.logger.Info("replaying playlists", zap.Int64("total", total))

	sem := make(chan struct{}, r.cfg.Concurrency)
	batchStart := time.Now()
	var processed int64

	for offset := int64(0); offset < total; offset += int64(r.cfg.BatchSize) {
		if ctx.Err() != nil {
			break
		}

		rows, err := r.srcDB.Query(ctx, selectQ, r.cfg.BatchSize, offset)
		if err != nil {
			return fmt.Errorf("query playlists at offset %d: %w", offset, err)
		}

		playlists, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (sourcePlaylist, error) {
			var p sourcePlaylist
			err := row.Scan(
				&p.PlaylistID, &p.PlaylistOwnerID, &p.PlaylistName, &p.Description,
				&p.IsAlbum, &p.IsPrivate,
				&p.MetadataMultihash, &p.ImageSizesMultihash, &p.PlaylistContents,
				&p.ReleaseDate, &p.IsStreamGated, &p.StreamConditions,
			)
			return p, err
		})
		if err != nil {
			return fmt.Errorf("scan playlists: %w", err)
		}

		for _, p := range playlists {
			if ctx.Err() != nil {
				break
			}
			p := p

			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()
				if err := r.submitPlaylist(ctx, p); err != nil {
					if ctx.Err() == nil {
						r.logger.Warn("playlist tx error", zap.Int64("playlist_id", p.PlaylistID), zap.Error(err))
						r.stats["playlists"].Errors++
					}
				} else {
					r.stats["playlists"].Submitted++
				}
			}()

			processed++
		}

		if processed%10000 == 0 {
			r.logProgress("playlists", processed, total, batchStart)
		}
	}

	for i := 0; i < r.cfg.Concurrency; i++ {
		sem <- struct{}{}
	}

	r.logProgress("playlists", processed, total, batchStart)
	return nil
}

func (r *Replayer) submitPlaylist(ctx context.Context, p sourcePlaylist) error {
	inner := playlistMetadataInner{
		IsAlbum:       p.IsAlbum,
		IsPrivate:     p.IsPrivate,
		IsStreamGated: p.IsStreamGated,
	}
	if p.PlaylistName != nil {
		inner.PlaylistName = *p.PlaylistName
	}
	if p.Description != nil {
		inner.Description = *p.Description
	}
	if p.ImageSizesMultihash != nil {
		inner.PlaylistImageSizesHash = *p.ImageSizesMultihash
	}
	if p.ReleaseDate != nil {
		inner.ReleaseDate = *p.ReleaseDate
	}
	if len(p.PlaylistContents) > 0 {
		var v interface{}
		if err := json.Unmarshal(p.PlaylistContents, &v); err == nil {
			inner.PlaylistContents = v
		}
	}
	if len(p.StreamConditions) > 0 {
		var v interface{}
		if err := json.Unmarshal(p.StreamConditions, &v); err == nil {
			inner.StreamConditions = v
		}
	}

	cid := "genesis-import"
	if p.MetadataMultihash != nil && *p.MetadataMultihash != "" {
		cid = *p.MetadataMultihash
	}

	wrapper := playlistMetadataWrapper{CID: cid, Data: inner}
	metaJSON, err := json.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("marshal playlist metadata: %w", err)
	}

	return r.submitManageEntity(ctx, &corev1.ManageEntityLegacy{
		UserId:     p.PlaylistOwnerID,
		EntityType: "Playlist",
		EntityId:   p.PlaylistID,
		Action:     "Create",
		Metadata:   string(metaJSON),
	})
}
