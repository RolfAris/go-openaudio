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

// userMetadataWrapper is the CID-wrapped format the DP expects for User CREATE.
// User CREATE uses parse_metadata(..., Action.UPDATE, ...) internally, which
// requires the same {"cid": "...", "data": {...}} envelope as tracks and playlists.
type userMetadataWrapper struct {
	CID  string       `json:"cid"`
	Data userMetadata `json:"data"`
}

// userMetadata mirrors the fields the discovery provider expects for User CREATE/UPDATE.
type userMetadata struct {
	Name                  string `json:"name,omitempty"`
	Handle                string `json:"handle,omitempty"`
	Bio                   string `json:"bio,omitempty"`
	Location              string `json:"location,omitempty"`
	Wallet                string `json:"wallet,omitempty"`
	ProfilePicture        string `json:"profile_picture,omitempty"`
	ProfilePictureSizes   string `json:"profile_picture_sizes,omitempty"`
	CoverPhoto            string `json:"cover_photo,omitempty"`
	CoverPhotoSizes       string `json:"cover_photo_sizes,omitempty"`
	TwitterHandle         string `json:"twitter_handle,omitempty"`
	InstagramHandle       string `json:"instagram_handle,omitempty"`
	Website               string `json:"website,omitempty"`
	IsVerified            bool   `json:"is_verified,omitempty"`
	AllowAiAttribution    bool   `json:"allow_ai_attribution,omitempty"`
	SplUsdcPayoutWallet   string `json:"spl_usdc_payout_wallet,omitempty"`
}

// sourceUser is a row from the discovery provider's users table.
type sourceUser struct {
	UserID              int64
	Wallet              *string
	Handle              *string
	Name                *string
	Bio                 *string
	Location            *string
	ProfilePicture      *string
	ProfilePictureSizes *string
	CoverPhoto          *string
	CoverPhotoSizes     *string
	TwitterHandle       *string
	InstagramHandle     *string
	Website             *string
	IsVerified          bool
}

func (r *Replayer) replayUsers(ctx context.Context) error {
	const countQ = `SELECT count(*) FROM users WHERE is_current = true AND is_deactivated = false AND is_available = true`
	const selectQ = `
		SELECT
			user_id, wallet, handle, name, bio, location,
			profile_picture, profile_picture_sizes,
			cover_photo, cover_photo_sizes,
			twitter_handle, instagram_handle, website,
			is_verified
		FROM users
		WHERE is_current = true AND is_deactivated = false AND is_available = true
		ORDER BY user_id
		LIMIT $1 OFFSET $2`

	var total int64
	if err := r.srcDB.QueryRow(ctx, countQ).Scan(&total); err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	r.logger.Info("replaying users", zap.Int64("total", total))

	sem := make(chan struct{}, r.cfg.Concurrency)
	batchStart := time.Now()
	var processed int64

	for offset := int64(0); offset < total; offset += int64(r.cfg.BatchSize) {
		if ctx.Err() != nil {
			break
		}

		rows, err := r.srcDB.Query(ctx, selectQ, r.cfg.BatchSize, offset)
		if err != nil {
			return fmt.Errorf("query users at offset %d: %w", offset, err)
		}

		users, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (sourceUser, error) {
			var u sourceUser
			err := row.Scan(
				&u.UserID, &u.Wallet, &u.Handle, &u.Name, &u.Bio, &u.Location,
				&u.ProfilePicture, &u.ProfilePictureSizes,
				&u.CoverPhoto, &u.CoverPhotoSizes,
				&u.TwitterHandle, &u.InstagramHandle, &u.Website,
				&u.IsVerified,
			)
			return u, err
		})
		if err != nil {
			return fmt.Errorf("scan users: %w", err)
		}

		for _, u := range users {
			if ctx.Err() != nil {
				break
			}
			u := u // capture

			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()
				if err := r.submitUser(ctx, u); err != nil {
					if ctx.Err() == nil {
						r.logger.Warn("user tx error", zap.Int64("user_id", u.UserID), zap.Error(err))
						r.stats["users"].Errors++
					}
				} else {
					r.stats["users"].Submitted++
				}
			}()

			processed++
		}

		if processed%10000 == 0 {
			r.logProgress("users", processed, total, batchStart)
		}
	}

	// drain
	for i := 0; i < r.cfg.Concurrency; i++ {
		sem <- struct{}{}
	}

	r.logProgress("users", processed, total, batchStart)
	return nil
}

func (r *Replayer) submitUser(ctx context.Context, u sourceUser) error {
	meta := userMetadata{
		IsVerified: u.IsVerified,
	}
	if u.Wallet != nil {
		meta.Wallet = *u.Wallet
	}
	if u.Handle != nil {
		meta.Handle = *u.Handle
	}
	if u.Name != nil {
		meta.Name = *u.Name
	}
	if u.Bio != nil {
		meta.Bio = *u.Bio
	}
	if u.Location != nil {
		meta.Location = *u.Location
	}
	if u.ProfilePicture != nil {
		meta.ProfilePicture = *u.ProfilePicture
	}
	if u.ProfilePictureSizes != nil {
		meta.ProfilePictureSizes = *u.ProfilePictureSizes
	}
	if u.CoverPhoto != nil {
		meta.CoverPhoto = *u.CoverPhoto
	}
	if u.CoverPhotoSizes != nil {
		meta.CoverPhotoSizes = *u.CoverPhotoSizes
	}
	if u.TwitterHandle != nil {
		meta.TwitterHandle = *u.TwitterHandle
	}
	if u.InstagramHandle != nil {
		meta.InstagramHandle = *u.InstagramHandle
	}
	if u.Website != nil {
		meta.Website = *u.Website
	}

	cid := "genesis-import"
	wrapper := userMetadataWrapper{CID: cid, Data: meta}
	metaJSON, err := json.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("marshal user metadata: %w", err)
	}

	return r.submitManageEntity(ctx, &corev1.ManageEntityLegacy{
		UserId:     u.UserID,
		EntityType: "User",
		EntityId:   u.UserID,
		Action:     "Create",
		Metadata:   string(metaJSON),
	})
}
