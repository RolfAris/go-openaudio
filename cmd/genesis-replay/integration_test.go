//go:build integration

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// testMigrationPrivKey is the private key for the genesis_migration_address
// configured in pkg/core/config/genesis/prod-v2.json.
// Address: 0xF7Bd9D733fAD4e2f0594bb88180cef8D8D03dEbB
// FOR LOCAL TESTING ONLY — this key has no value on any real network.
const testMigrationPrivKey = "fc972c220946cde07bf3c6be380cc956c3d5680a2d7b225c660866e450cf1299"

// TestGenesisReplay is a full end-to-end integration test.
//
// Prerequisites (all provided by docker-compose up in cmd/genesis-replay/):
//   - genesis_replay_source postgres DB seeded with testdata/seed.sql
//   - discovery_provider_1 postgres DB (empty except genesis block row)
//   - openaudio-1 node running at GENESIS_CHAIN_URL (default: https://node1.oap.devnet)
//   - discovery-provider service indexing the chain
//
// Optional env (defaults match docker-compose):
//
//	GENESIS_SRC_DSN    source DB DSN  (default: postgres://postgres:postgres@localhost:5432/genesis_replay_source)
//	GENESIS_DEST_DSN   dest DB DSN    (default: postgres://postgres:postgres@localhost:5432/discovery_provider_1)
//	GENESIS_CHAIN_URL  chain URL      (default: https://node1.oap.devnet)
//
// Run with:
//
//	go test -v -tags integration -run TestGenesisReplay -timeout 20m ./cmd/genesis-replay/...
func TestGenesisReplay(t *testing.T) {
	srcDSN := envOrDefault("GENESIS_SRC_DSN", "postgres://postgres:postgres@localhost:5434/genesis_replay_source?sslmode=disable")
	dstDSN := envOrDefault("GENESIS_DEST_DSN", "postgres://postgres:postgres@localhost:5434/discovery_provider_1?sslmode=disable")
	chainURL := envOrDefault("GENESIS_CHAIN_URL", "https://node1.oap.devnet")

	privKey, err := parsePrivKey(testMigrationPrivKey)
	require.NoError(t, err, "parse private key")

	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// 1. Connect to source DB and snapshot expected state.
	srcPool, err := pgxpool.New(ctx, srcDSN)
	require.NoError(t, err)
	defer srcPool.Close()
	require.NoError(t, srcPool.Ping(ctx), "ping source DB")

	expected := snapshotSource(t, ctx, srcPool)
	t.Logf("source snapshot: %d users, %d tracks, %d playlists, %d follows, %d saves, %d reposts",
		len(expected.users), len(expected.tracks), len(expected.playlists),
		len(expected.follows), len(expected.saves), len(expected.reposts))

	// 2. Run the replayer.
	cfg := &ReplayConfig{
		SrcDSN:      srcDSN,
		ChainURL:    chainURL,
		PrivKey:     privKey,
		Network:     "dev",
		Concurrency: 20,
		BatchSize:   100,
	}
	r, err := NewReplayer(cfg, logger)
	require.NoError(t, err, "init replayer")
	defer r.Close()

	require.NoError(t, r.Run(ctx), "run replayer")
	t.Log("replay submitted, waiting for discovery provider to index...")

	// 3. Connect to dest DB and wait for all replayed entity types to be indexed.
	dstPool, err := pgxpool.New(ctx, dstDSN)
	require.NoError(t, err)
	defer dstPool.Close()
	require.NoError(t, dstPool.Ping(ctx), "ping dest DB")

	waitForCounts(t, ctx, dstPool, expected)

	// 4. Compare source to indexed destination.
	t.Log("indexing converged, comparing data...")
	compareUsers(t, ctx, dstPool, expected.users)
	compareTracks(t, ctx, dstPool, expected.tracks)
	comparePlaylists(t, ctx, dstPool, expected.playlists)
	compareFollows(t, ctx, dstPool, expected.follows)
	compareSaves(t, ctx, dstPool, expected.saves)
	compareReposts(t, ctx, dstPool, expected.reposts)
}

// ---- helpers ----------------------------------------------------------------

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ---- source snapshot --------------------------------------------------------

type sourceSnapshot struct {
	users     []snapshotUser
	tracks    []snapshotTrack
	playlists []snapshotPlaylist
	follows   []followPair
	saves     []saveTuple
	reposts   []repostTuple
}

type snapshotUser struct {
	UserID   int64
	Handle   string
	Name     string
	Bio      string
	Location string
	// IsVerified is omitted: set by on-chain ETH verification, not replayable via entity manager.
}

type snapshotTrack struct {
	TrackID int64
	OwnerID int64
	Title   string
	Genre   string
}

type snapshotPlaylist struct {
	PlaylistID      int64
	PlaylistOwnerID int64
	PlaylistName    string
	IsAlbum         bool
}

type followPair struct{ Follower, Followee int64 }

type saveTuple struct {
	UserID   int64
	ItemID   int64
	SaveType string
}

type repostTuple struct {
	UserID     int64
	ItemID     int64
	RepostType string
}

func snapshotSource(t *testing.T, ctx context.Context, db *pgxpool.Pool) sourceSnapshot {
	t.Helper()
	var s sourceSnapshot

	s.users = queryUsers(t, ctx, db)
	s.tracks = queryTracks(t, ctx, db)
	s.playlists = queryPlaylists(t, ctx, db)
	s.follows = queryFollows(t, ctx, db)
	s.saves = querySaves(t, ctx, db)
	s.reposts = queryReposts(t, ctx, db)
	return s
}

func queryUsers(t *testing.T, ctx context.Context, db *pgxpool.Pool) []snapshotUser {
	t.Helper()
	rows, err := db.Query(ctx, `
		SELECT user_id,
		       COALESCE(handle, ''), COALESCE(name, ''),
		       COALESCE(bio, ''), COALESCE(location, '')
		FROM users
		WHERE is_current = true AND is_deactivated = false AND is_available = true
		ORDER BY user_id`)
	require.NoError(t, err)
	defer rows.Close()

	var out []snapshotUser
	for rows.Next() {
		var u snapshotUser
		require.NoError(t, rows.Scan(&u.UserID, &u.Handle, &u.Name, &u.Bio, &u.Location))
		out = append(out, u)
	}
	require.NoError(t, rows.Err())
	return out
}

func queryTracks(t *testing.T, ctx context.Context, db *pgxpool.Pool) []snapshotTrack {
	t.Helper()
	rows, err := db.Query(ctx, `
		SELECT track_id, owner_id, COALESCE(title, ''), COALESCE(genre, '')
		FROM tracks
		WHERE is_current = true AND is_delete = false AND is_available = true
		ORDER BY track_id`)
	require.NoError(t, err)
	defer rows.Close()

	var out []snapshotTrack
	for rows.Next() {
		var tr snapshotTrack
		require.NoError(t, rows.Scan(&tr.TrackID, &tr.OwnerID, &tr.Title, &tr.Genre))
		out = append(out, tr)
	}
	require.NoError(t, rows.Err())
	return out
}

func queryPlaylists(t *testing.T, ctx context.Context, db *pgxpool.Pool) []snapshotPlaylist {
	t.Helper()
	rows, err := db.Query(ctx, `
		SELECT playlist_id, playlist_owner_id, COALESCE(playlist_name, ''), is_album
		FROM playlists
		WHERE is_current = true AND is_delete = false
		ORDER BY playlist_id`)
	require.NoError(t, err)
	defer rows.Close()

	var out []snapshotPlaylist
	for rows.Next() {
		var p snapshotPlaylist
		require.NoError(t, rows.Scan(&p.PlaylistID, &p.PlaylistOwnerID, &p.PlaylistName, &p.IsAlbum))
		out = append(out, p)
	}
	require.NoError(t, rows.Err())
	return out
}

func queryFollows(t *testing.T, ctx context.Context, db *pgxpool.Pool) []followPair {
	t.Helper()
	rows, err := db.Query(ctx, `
		SELECT follower_user_id, followee_user_id
		FROM follows
		WHERE is_current = true AND is_delete = false
		ORDER BY follower_user_id, followee_user_id`)
	require.NoError(t, err)
	defer rows.Close()

	var out []followPair
	for rows.Next() {
		var f followPair
		require.NoError(t, rows.Scan(&f.Follower, &f.Followee))
		out = append(out, f)
	}
	require.NoError(t, rows.Err())
	return out
}

func querySaves(t *testing.T, ctx context.Context, db *pgxpool.Pool) []saveTuple {
	t.Helper()
	// The DP normalizes save_type 'album' → 'playlist'; match that here.
	rows, err := db.Query(ctx, `
		SELECT user_id, save_item_id,
		       CASE save_type WHEN 'album' THEN 'playlist' ELSE save_type END
		FROM saves
		WHERE is_current = true AND is_delete = false
		ORDER BY user_id, save_item_id`)
	require.NoError(t, err)
	defer rows.Close()

	var out []saveTuple
	for rows.Next() {
		var s saveTuple
		require.NoError(t, rows.Scan(&s.UserID, &s.ItemID, &s.SaveType))
		out = append(out, s)
	}
	require.NoError(t, rows.Err())
	return out
}

func queryReposts(t *testing.T, ctx context.Context, db *pgxpool.Pool) []repostTuple {
	t.Helper()
	rows, err := db.Query(ctx, `
		SELECT user_id, repost_item_id, repost_type
		FROM reposts
		WHERE is_current = true AND is_delete = false
		ORDER BY user_id, repost_item_id`)
	require.NoError(t, err)
	defer rows.Close()

	var out []repostTuple
	for rows.Next() {
		var rp repostTuple
		require.NoError(t, rows.Scan(&rp.UserID, &rp.ItemID, &rp.RepostType))
		out = append(out, rp)
	}
	require.NoError(t, rows.Err())
	return out
}

// ---- indexing wait ----------------------------------------------------------

// waitForCounts polls the destination DB until all entity counts reach the
// expected values, or until the context deadline.
func waitForCounts(t *testing.T, ctx context.Context, db *pgxpool.Pool, exp sourceSnapshot) {
	t.Helper()
	type check struct {
		name  string
		want  int
		query string
	}
	checks := []check{
		{
			"users",
			len(exp.users),
			`SELECT count(*) FROM users WHERE is_current=true AND is_deactivated=false AND is_available=true`,
		},
		{
			"tracks",
			len(exp.tracks),
			`SELECT count(*) FROM tracks WHERE is_current=true AND is_delete=false AND is_available=true`,
		},
		{
			"playlists",
			len(exp.playlists),
			`SELECT count(*) FROM playlists WHERE is_current=true AND is_delete=false`,
		},
		{
			"follows",
			len(exp.follows),
			`SELECT count(*) FROM follows WHERE is_current=true AND is_delete=false`,
		},
		{
			"saves",
			len(exp.saves),
			`SELECT count(*) FROM saves WHERE is_current=true AND is_delete=false`,
		},
		{
			"reposts",
			len(exp.reposts),
			`SELECT count(*) FROM reposts WHERE is_current=true AND is_delete=false`,
		},
	}

	deadline := time.Now().Add(10 * time.Minute)
	for {
		if ctx.Err() != nil {
			t.Fatal("context cancelled while waiting for indexing")
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for discovery provider indexing to converge")
		}

		allDone := true
		for _, c := range checks {
			var got int
			if err := db.QueryRow(ctx, c.query).Scan(&got); err != nil {
				t.Logf("poll %s: query error: %v", c.name, err)
				allDone = false
				continue
			}
			if got < c.want {
				t.Logf("poll %s: %d / %d", c.name, got, c.want)
				allDone = false
			}
		}
		if allDone {
			t.Log("all counts reached expected values")
			return
		}
		time.Sleep(5 * time.Second)
	}
}

// ---- comparisons ------------------------------------------------------------

func compareUsers(t *testing.T, ctx context.Context, db *pgxpool.Pool, expected []snapshotUser) {
	t.Helper()
	got := queryUsers(t, ctx, db)
	assert.Equal(t, expected, got, "users mismatch")
}

func compareTracks(t *testing.T, ctx context.Context, db *pgxpool.Pool, expected []snapshotTrack) {
	t.Helper()
	got := queryTracks(t, ctx, db)
	assert.Equal(t, expected, got, "tracks mismatch")
}

func comparePlaylists(t *testing.T, ctx context.Context, db *pgxpool.Pool, expected []snapshotPlaylist) {
	t.Helper()
	got := queryPlaylists(t, ctx, db)
	assert.Equal(t, expected, got, "playlists mismatch")
}

func compareFollows(t *testing.T, ctx context.Context, db *pgxpool.Pool, expected []followPair) {
	t.Helper()
	got := queryFollows(t, ctx, db)
	assert.Equal(t, expected, got, "follows mismatch")
}

func compareSaves(t *testing.T, ctx context.Context, db *pgxpool.Pool, expected []saveTuple) {
	t.Helper()
	got := querySaves(t, ctx, db)
	assert.Equal(t, expected, got, "saves mismatch")
}

func compareReposts(t *testing.T, ctx context.Context, db *pgxpool.Pool, expected []repostTuple) {
	t.Helper()
	got := queryReposts(t, ctx, db)
	assert.Equal(t, expected, got, "reposts mismatch")
}
