package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

func verifyCmd() *cli.Command {
	return &cli.Command{
		Name:  "verify",
		Usage: "Diff entities and plays between two discovery-provider databases",
		Description: `Streams both databases in sorted order and performs a merge comparison.
Plays are compared by per-track aggregate count rather than individual rows,
so the command is safe to run against a full production dataset.

Exit code 0 = all checks pass; 1 = mismatches found; 2 = fatal error.`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "src",
				Usage:    "Source (reference) PostgreSQL DSN",
				EnvVars:  []string{"GENESIS_SRC_DSN"},
				Required: true,
			},
			&cli.StringFlag{
				Name:     "dst",
				Usage:    "Destination PostgreSQL DSN",
				EnvVars:  []string{"GENESIS_DEST_DSN"},
				Required: true,
			},
			&cli.IntFlag{
				Name:    "max-samples",
				Usage:   "Maximum mismatch rows to print per entity type",
				EnvVars: []string{"GENESIS_MAX_SAMPLES"},
				Value:   10,
			},
			&cli.BoolFlag{
				Name:    "skip-plays",
				Usage:   "Skip play-count verification",
				EnvVars: []string{"GENESIS_SKIP_PLAYS"},
			},
		},
		Action: runVerify,
	}
}

// ---- result type ------------------------------------------------------------

type diffResult struct {
	entity    string
	srcCount  int64
	dstCount  int64
	missing   int64  // in src, absent from dst
	extra     int64  // in dst, absent from src
	different int64  // same key, differing values
	samples   []string
}

func (r *diffResult) ok() bool {
	return r.missing == 0 && r.extra == 0 && r.different == 0
}

// ---- entry point ------------------------------------------------------------

func runVerify(c *cli.Context) error {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	maxSamples := c.Int("max-samples")

	src, err := pgxpool.New(ctx, c.String("src"))
	if err != nil {
		return fmt.Errorf("connect src: %w", err)
	}
	defer src.Close()

	dst, err := pgxpool.New(ctx, c.String("dst"))
	if err != nil {
		return fmt.Errorf("connect dst: %w", err)
	}
	defer dst.Close()

	v := &verifier{src: src, dst: dst, maxSamples: maxSamples, logger: logger}

	type check struct {
		name string
		skip bool
		fn   func(context.Context) (*diffResult, error)
	}
	checks := []check{
		{"users", false, v.verifyUsers},
		{"tracks", false, v.verifyTracks},
		{"playlists", false, v.verifyPlaylists},
		{"follows", false, v.verifyFollows},
		{"saves", false, v.verifySaves},
		{"reposts", false, v.verifyReposts},
		{"plays", c.Bool("skip-plays"), v.verifyPlays},
	}

	// Run all checks first, then print everything at once.
	results := make([]*diffResult, len(checks))
	allOK := true
	for i, chk := range checks {
		if chk.skip {
			results[i] = &diffResult{entity: chk.name}
			continue
		}
		logger.Info("verifying", zap.String("entity", chk.name))
		r, err := chk.fn(ctx)
		if err != nil {
			return fmt.Errorf("verify %s: %w", chk.name, err)
		}
		results[i] = r
		if !r.ok() {
			allOK = false
		}
	}

	// Summary table.
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "entity\tsrc\tdst\tmissing\textra\tdifferent\tstatus")
	fmt.Fprintln(tw, "------\t---\t---\t-------\t-----\t---------\t------")
	for i, r := range results {
		if checks[i].skip {
			fmt.Fprintf(tw, "%s\t-\t-\t-\t-\t-\tSKIPPED\n", r.entity)
			continue
		}
		status := "OK"
		if !r.ok() {
			status = "FAIL"
		}
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%s\n",
			r.entity, r.srcCount, r.dstCount, r.missing, r.extra, r.different, status)
	}
	tw.Flush()

	// Per-entity sample mismatches.
	for _, r := range results {
		if len(r.samples) > 0 {
			fmt.Printf("\n--- %s mismatches (sample) ---\n", r.entity)
			for _, s := range r.samples {
				fmt.Println(" ", s)
			}
		}
	}

	if !allOK {
		os.Exit(1)
	}
	return nil
}

// ---- verifier ---------------------------------------------------------------

type verifier struct {
	src        *pgxpool.Pool
	dst        *pgxpool.Pool
	maxSamples int
	logger     *zap.Logger
}

// compareStreams opens both queries and performs a streaming merge comparison.
// Both queries MUST return rows in the same ascending key order.
// keyLen is the number of leading columns forming the primary key;
// remaining columns are value fields checked for equality.
//
// Memory usage is O(maxSamples) — only two rows are held in memory at a time.
func (v *verifier) compareStreams(ctx context.Context, name, srcQ, dstQ string, keyLen int) (*diffResult, error) {
	res := &diffResult{entity: name}

	srcRows, err := v.src.Query(ctx, srcQ)
	if err != nil {
		return nil, fmt.Errorf("src: %w", err)
	}
	defer srcRows.Close()

	dstRows, err := v.dst.Query(ctx, dstQ)
	if err != nil {
		return nil, fmt.Errorf("dst: %w", err)
	}
	defer dstRows.Close()

	// Lazy-load state: advance the cursor, then load values on demand.
	// This lets us "put back" the row that's ahead without re-reading the DB.
	hasSrc := srcRows.Next()
	hasDst := dstRows.Next()
	var srcVals, dstVals []any // nil means "not yet loaded for this position"

	for hasSrc || hasDst {
		// Load current values if not yet read.
		if hasSrc && srcVals == nil {
			if srcVals, err = srcRows.Values(); err != nil {
				return nil, fmt.Errorf("src scan: %w", err)
			}
			res.srcCount++
		}
		if hasDst && dstVals == nil {
			if dstVals, err = dstRows.Values(); err != nil {
				return nil, fmt.Errorf("dst scan: %w", err)
			}
			res.dstCount++
		}

		switch {
		case !hasDst:
			// dst exhausted — everything remaining in src is missing.
			res.missing++
			v.addSample(res, fmt.Sprintf("MISSING key=%s", rowKey(srcVals, keyLen)))
			srcVals = nil
			hasSrc = srcRows.Next()

		case !hasSrc:
			// src exhausted — everything remaining in dst is extra.
			res.extra++
			v.addSample(res, fmt.Sprintf("EXTRA   key=%s", rowKey(dstVals, keyLen)))
			dstVals = nil
			hasDst = dstRows.Next()

		default:
			srcKey := rowKey(srcVals, keyLen)
			dstKey := rowKey(dstVals, keyLen)

			switch {
			case srcKey < dstKey:
				// src row not in dst.
				res.missing++
				v.addSample(res, fmt.Sprintf("MISSING key=%s", srcKey))
				srcVals = nil        // advance src
				hasSrc = srcRows.Next()
				// dstVals stays — it hasn't been "used" yet, just peeked at.
				// But res.dstCount was already incremented when we loaded it;
				// undo that so counts only reflect actually-compared dst rows.
				res.dstCount--

			case srcKey > dstKey:
				// dst row not in src.
				res.extra++
				v.addSample(res, fmt.Sprintf("EXTRA   key=%s", dstKey))
				dstVals = nil        // advance dst
				hasDst = dstRows.Next()
				res.srcCount-- // undo premature src count

			default:
				// Keys match — compare values.
				if diff := rowValueDiff(srcVals, dstVals, keyLen); diff != "" {
					res.different++
					v.addSample(res, fmt.Sprintf("DIFF    key=%s %s", srcKey, diff))
				}
				srcVals = nil
				dstVals = nil
				hasSrc = srcRows.Next()
				hasDst = dstRows.Next()
			}
		}
	}

	if err := srcRows.Err(); err != nil {
		return nil, fmt.Errorf("src rows: %w", err)
	}
	if err := dstRows.Err(); err != nil {
		return nil, fmt.Errorf("dst rows: %w", err)
	}
	return res, nil
}

func (v *verifier) addSample(r *diffResult, s string) {
	if len(r.samples) < v.maxSamples {
		r.samples = append(r.samples, s)
	}
}

// ---- helpers ----------------------------------------------------------------

// rowKey formats the first keyLen values as a colon-separated string.
// Integers are zero-padded to 20 digits so lexicographic order matches numeric order.
func rowKey(vals []any, keyLen int) string {
	parts := make([]string, keyLen)
	for i := range keyLen {
		switch n := vals[i].(type) {
		case int32:
			parts[i] = fmt.Sprintf("%020d", n)
		case int64:
			parts[i] = fmt.Sprintf("%020d", n)
		default:
			parts[i] = fmt.Sprintf("%v", n)
		}
	}
	return strings.Join(parts, ":")
}

// rowValueDiff returns a description of value differences, or "" if equal.
func rowValueDiff(src, dst []any, keyLen int) string {
	var diffs []string
	for i := keyLen; i < len(src); i++ {
		sv := fmt.Sprintf("%v", src[i])
		dv := fmt.Sprintf("%v", dst[i])
		if sv != dv {
			diffs = append(diffs, fmt.Sprintf("col%d: %q→%q", i, sv, dv))
		}
	}
	return strings.Join(diffs, ", ")
}

// ---- entity checks ----------------------------------------------------------

func (v *verifier) verifyUsers(ctx context.Context) (*diffResult, error) {
	q := `SELECT user_id,
	             COALESCE(handle,''), COALESCE(name,''),
	             COALESCE(bio,''), COALESCE(location,'')
	      FROM users
	      WHERE is_current = true AND is_deactivated = false AND is_available = true
	      ORDER BY user_id`
	return v.compareStreams(ctx, "users", q, q, 1)
}

func (v *verifier) verifyTracks(ctx context.Context) (*diffResult, error) {
	q := `SELECT track_id, owner_id, COALESCE(title,''), COALESCE(genre,'')
	      FROM tracks
	      WHERE is_current = true AND is_delete = false AND is_available = true
	      ORDER BY track_id`
	return v.compareStreams(ctx, "tracks", q, q, 1)
}

func (v *verifier) verifyPlaylists(ctx context.Context) (*diffResult, error) {
	q := `SELECT playlist_id, playlist_owner_id, COALESCE(playlist_name,''), is_album
	      FROM playlists
	      WHERE is_current = true AND is_delete = false
	      ORDER BY playlist_id`
	return v.compareStreams(ctx, "playlists", q, q, 1)
}

func (v *verifier) verifyFollows(ctx context.Context) (*diffResult, error) {
	q := `SELECT follower_user_id, followee_user_id
	      FROM follows
	      WHERE is_current = true AND is_delete = false
	      ORDER BY follower_user_id, followee_user_id`
	return v.compareStreams(ctx, "follows", q, q, 2)
}

func (v *verifier) verifySaves(ctx context.Context) (*diffResult, error) {
	// Normalize 'album' → 'playlist': the DP stores album saves under the playlist type.
	q := `SELECT user_id, save_item_id,
	             CASE save_type WHEN 'album' THEN 'playlist' ELSE save_type END
	      FROM saves
	      WHERE is_current = true AND is_delete = false
	      ORDER BY user_id, save_item_id`
	return v.compareStreams(ctx, "saves", q, q, 2)
}

func (v *verifier) verifyReposts(ctx context.Context) (*diffResult, error) {
	q := `SELECT user_id, repost_item_id, repost_type
	      FROM reposts
	      WHERE is_current = true AND is_delete = false
	      ORDER BY user_id, repost_item_id`
	return v.compareStreams(ctx, "reposts", q, q, 2)
}

// verifyPlays compares per-track play counts rather than individual play rows.
// Memory and query cost scale with the number of distinct tracks, not plays.
func (v *verifier) verifyPlays(ctx context.Context) (*diffResult, error) {
	q := `SELECT play_item_id::bigint, count(*)::bigint
	      FROM plays
	      GROUP BY play_item_id
	      ORDER BY play_item_id`
	return v.compareStreams(ctx, "plays", q, q, 1)
}
