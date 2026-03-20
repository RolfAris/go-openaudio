package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"

	"connectrpc.com/connect"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	corev1connect "github.com/OpenAudio/go-openaudio/pkg/api/core/v1/v1connect"
	"github.com/OpenAudio/go-openaudio/pkg/core/config"
	"github.com/OpenAudio/go-openaudio/pkg/core/server"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ReplayConfig holds configuration for the replay tool.
type ReplayConfig struct {
	SrcDSN        string
	ChainURL      string
	PrivKey       *ecdsa.PrivateKey
	Network       string
	Concurrency   int
	BatchSize     int
	SkipUsers     bool
	SkipTracks    bool
	SkipPlaylists bool
	SkipSocial    bool
	SkipPlays     bool
}

// EntityStats tracks progress per entity type.
type EntityStats struct {
	Submitted int64
	Errors    int64
}

// Replayer orchestrates the genesis replay.
type Replayer struct {
	cfg        *ReplayConfig
	srcDB      *pgxpool.Pool
	chain      corev1connect.CoreServiceClient
	sigConfig  *config.Config
	privKey    *ecdsa.PrivateKey
	signerAddr string
	nonce      atomic.Uint64
	logger     *zap.Logger
	stats      map[string]*EntityStats
}

// NewReplayer creates and connects a Replayer.
func NewReplayer(cfg *ReplayConfig, logger *zap.Logger) (*Replayer, error) {
	srcDB, err := pgxpool.New(context.Background(), cfg.SrcDSN)
	if err != nil {
		return nil, fmt.Errorf("connect src db: %w", err)
	}
	if err := srcDB.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping src db: %w", err)
	}

	chainURL := cfg.ChainURL
	if !strings.HasPrefix(chainURL, "http://") && !strings.HasPrefix(chainURL, "https://") {
		chainURL = "https://" + chainURL
	}

	var httpClient *http.Client
	if strings.HasPrefix(chainURL, "http://") {
		// h2c: plain HTTP/2 without TLS, used to bypass nginx and talk directly to gRPC port
		httpClient = &http.Client{
			Transport: &http2.Transport{
				AllowHTTP: true,
				DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
					return net.Dial(network, addr)
				},
			},
			Timeout: 60 * time.Second,
		}
	} else {
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			Timeout: 60 * time.Second,
		}
	}
	chain := corev1connect.NewCoreServiceClient(httpClient, chainURL)

	sigCfg := signingConfig(cfg.Network)

	addr := crypto.PubkeyToAddress(cfg.PrivKey.PublicKey)
	logger.Info("genesis migration keypair loaded",
		zap.String("address", addr.Hex()),
		zap.String("network", cfg.Network),
		zap.String("acdc_address", sigCfg.AcdcEntityManagerAddress),
		zap.Uint("acdc_chain_id", uint(sigCfg.AcdcChainID)),
	)

	r := &Replayer{
		cfg:        cfg,
		srcDB:      srcDB,
		chain:      chain,
		sigConfig:  sigCfg,
		privKey:    cfg.PrivKey,
		signerAddr: addr.Hex(),
		logger:     logger,
		stats:      map[string]*EntityStats{},
	}
	for _, et := range []string{"users", "tracks", "playlists", "follows", "saves", "reposts", "plays"} {
		r.stats[et] = &EntityStats{}
	}
	return r, nil
}

// Close releases resources.
func (r *Replayer) Close() {
	r.srcDB.Close()
}

// Run executes the full replay in dependency order.
func (r *Replayer) Run(ctx context.Context) error {
	start := time.Now()
	r.logger.Info("starting genesis replay")

	steps := []struct {
		name string
		skip bool
		fn   func(context.Context) error
	}{
		{"users", r.cfg.SkipUsers, r.replayUsers},
		{"tracks", r.cfg.SkipTracks, r.replayTracks},
		{"playlists", r.cfg.SkipPlaylists, r.replayPlaylists},
		{"social (follows/saves/reposts)", r.cfg.SkipSocial, r.replaySocial},
		{"plays", r.cfg.SkipPlays, r.replayPlays},
	}

	for _, step := range steps {
		if step.skip {
			r.logger.Info("skipping", zap.String("step", step.name))
			continue
		}
		r.logger.Info("replaying", zap.String("step", step.name))
		if err := step.fn(ctx); err != nil {
			if ctx.Err() != nil {
				r.logger.Info("replay interrupted by signal")
				break
			}
			return fmt.Errorf("replay %s: %w", step.name, err)
		}
	}

	r.printSummary(time.Since(start))
	return nil
}

// submitManageEntity signs and forwards a ManageEntityLegacy transaction.
// Retries once with backoff on mempool-full errors.
func (r *Replayer) submitManageEntity(ctx context.Context, me *corev1.ManageEntityLegacy) error {
	me.Nonce = r.nextNonce()

	if err := server.SignManageEntity(r.sigConfig, me, r.privKey); err != nil {
		return fmt.Errorf("sign: %w", err)
	}

	req := &corev1.ForwardTransactionRequest{
		Transaction: &corev1.SignedTransaction{
			RequestId: uuid.NewString(),
			Transaction: &corev1.SignedTransaction_ManageEntity{
				ManageEntity: me,
			},
		},
	}

	return r.forwardWithRetry(ctx, req)
}

// forwardWithRetry submits a transaction, retrying on mempool-full with backoff.
func (r *Replayer) forwardWithRetry(ctx context.Context, req *corev1.ForwardTransactionRequest) error {
	for attempt := 0; attempt < 5; attempt++ {
		_, err := r.chain.ForwardTransaction(ctx, connect.NewRequest(req))
		if err == nil {
			return nil
		}
		if !strings.Contains(err.Error(), "mempool full") {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(500*(attempt+1)) * time.Millisecond):
		}
	}
	return fmt.Errorf("mempool full after retries")
}

// nextNonce returns a unique 32-byte hex-encoded nonce.
func (r *Replayer) nextNonce() string {
	n := r.nonce.Add(1)
	b := make([]byte, 32)
	b[24] = byte(n >> 56)
	b[25] = byte(n >> 48)
	b[26] = byte(n >> 40)
	b[27] = byte(n >> 32)
	b[28] = byte(n >> 24)
	b[29] = byte(n >> 16)
	b[30] = byte(n >> 8)
	b[31] = byte(n)
	return "0x" + hex.EncodeToString(b)
}

// signingConfig returns the EIP712 signing config for the given network.
func signingConfig(network string) *config.Config {
	switch network {
	case "prod", "production", "mainnet":
		return &config.Config{
			AcdcEntityManagerAddress: config.ProdAcdcAddress,
			AcdcChainID:              config.ProdAcdcChainID,
		}
	case "stage", "staging":
		return &config.Config{
			AcdcEntityManagerAddress: config.StageAcdcAddress,
			AcdcChainID:              config.StageAcdcChainID,
		}
	default:
		return &config.Config{
			AcdcEntityManagerAddress: config.DevAcdcAddress,
			AcdcChainID:              config.DevAcdcChainID,
		}
	}
}

// withConcurrency processes items concurrently up to r.cfg.Concurrency.
// work is a function that receives a semaphore-style token; caller must call
// release() when done. errors are logged and counted; the function does not stop on error.
func (r *Replayer) withConcurrency(ctx context.Context, entityType string, work func(ctx context.Context) error) func() {
	sem := make(chan struct{}, r.cfg.Concurrency)
	return func() {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return
		}
		go func() {
			defer func() { <-sem }()
			if err := work(ctx); err != nil {
				if ctx.Err() == nil {
					r.logger.Warn("tx error", zap.String("type", entityType), zap.Error(err))
					atomic.AddInt64(&r.stats[entityType].Errors, 1)
				}
			} else {
				atomic.AddInt64(&r.stats[entityType].Submitted, 1)
			}
		}()
	}
}

// drainSemaphore waits for all in-flight goroutines to finish.
func drainSemaphore(sem chan struct{}, concurrency int) {
	for i := 0; i < concurrency; i++ {
		sem <- struct{}{}
	}
}

// logProgress logs periodic progress for a batch.
func (r *Replayer) logProgress(entityType string, processed, total int64, batchStart time.Time) {
	elapsed := time.Since(batchStart).Seconds()
	rate := float64(processed) / elapsed
	r.logger.Info("progress",
		zap.String("type", entityType),
		zap.Int64("processed", processed),
		zap.Int64("total", total),
		zap.String("rate", fmt.Sprintf("%.0f/s", rate)),
	)
}

func (r *Replayer) printSummary(elapsed time.Duration) {
	r.logger.Info("replay complete", zap.Duration("elapsed", elapsed))
	for entityType, s := range r.stats {
		r.logger.Info("stats",
			zap.String("type", entityType),
			zap.Int64("submitted", s.Submitted),
			zap.Int64("errors", s.Errors),
		)
	}
}
