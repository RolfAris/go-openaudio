// cmd/genesis-replay: submits synthetic ManageEntity and TrackPlay transactions
// to a bootstrap chain to seed it with the full historical Audius state.
//
// Usage:
//
//	genesis-replay keygen
//	  Generates a new Ethereum keypair and prints the address and private key.
//	  Set the printed address as genesis_migration_address in prod-v2.json.
//
//	genesis-replay run --src-dsn <postgres_dsn> --chain-url <url> --private-key <hex>
//	  Replays all entities from the source database to the bootstrap chain.
package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

func main() {
	app := &cli.App{
		Name:  "genesis-replay",
		Usage: "Seed a bootstrap Core chain with historical Audius state",
		Commands: []*cli.Command{
			keygenCmd(),
			runCmd(),
			verifyCmd(),
		},
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func keygenCmd() *cli.Command {
	return &cli.Command{
		Name:  "keygen",
		Usage: "Generate a genesis migration keypair",
		Action: func(c *cli.Context) error {
			privKey, err := crypto.GenerateKey()
			if err != nil {
				return fmt.Errorf("generate key: %w", err)
			}
			addr := crypto.PubkeyToAddress(privKey.PublicKey)
			privBytes := privKey.D.Bytes()
			// pad to 32 bytes
			padded := make([]byte, 32)
			copy(padded[32-len(privBytes):], privBytes)
			fmt.Printf("Address:     %s\n", addr.Hex())
			fmt.Printf("PrivateKey:  0x%s\n", hex.EncodeToString(padded))
			fmt.Println()
			fmt.Println("Set genesis_migration_address in pkg/core/config/genesis/prod-v2.json to the address above.")
			fmt.Println("Pass the private key to 'genesis-replay run --private-key <hex>'.")
			return nil
		},
	}
}

func runCmd() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "Replay entities from source DB to bootstrap chain",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "src-dsn",
				Usage:    "Source PostgreSQL DSN (the DB dump)",
				EnvVars:  []string{"GENESIS_SRC_DSN"},
				Required: true,
			},
			&cli.StringFlag{
				Name:    "chain-url",
				Usage:   "Bootstrap chain URL (e.g. http://localhost:50051 for direct gRPC, https://node1.oap.devnet for HTTPS)",
				EnvVars: []string{"GENESIS_CHAIN_URL"},
				Value:   "http://localhost:50051",
			},
			&cli.StringFlag{
				Name:     "private-key",
				Usage:    "Genesis migration private key (hex, with or without 0x prefix)",
				EnvVars:  []string{"GENESIS_MIGRATION_PRIVATE_KEY"},
				Required: true,
			},
			&cli.StringFlag{
				Name:    "network",
				Usage:   "Network environment: prod, stage, dev",
				EnvVars: []string{"NETWORK"},
				Value:   "prod",
			},
			&cli.IntFlag{
				Name:    "concurrency",
				Usage:   "Number of concurrent transaction submissions",
				EnvVars: []string{"GENESIS_CONCURRENCY"},
				Value:   500,
			},
			&cli.IntFlag{
				Name:    "batch-size",
				Usage:   "Rows fetched from source DB per batch",
				EnvVars: []string{"GENESIS_BATCH_SIZE"},
				Value:   1000,
			},
			&cli.BoolFlag{Name: "skip-users", EnvVars: []string{"GENESIS_SKIP_USERS"}},
			&cli.BoolFlag{Name: "skip-tracks", EnvVars: []string{"GENESIS_SKIP_TRACKS"}},
			&cli.BoolFlag{Name: "skip-playlists", EnvVars: []string{"GENESIS_SKIP_PLAYLISTS"}},
			&cli.BoolFlag{Name: "skip-social", EnvVars: []string{"GENESIS_SKIP_SOCIAL"}},
			&cli.BoolFlag{Name: "skip-plays", EnvVars: []string{"GENESIS_SKIP_PLAYS"}},
		},
		Action: func(c *cli.Context) error {
			privKey, err := parsePrivKey(c.String("private-key"))
			if err != nil {
				return fmt.Errorf("parse private key: %w", err)
			}

			logger, _ := zap.NewProduction()
			defer logger.Sync()

			cfg := &ReplayConfig{
				SrcDSN:        c.String("src-dsn"),
				ChainURL:      c.String("chain-url"),
				PrivKey:       privKey,
				Network:       c.String("network"),
				Concurrency:   c.Int("concurrency"),
				BatchSize:     c.Int("batch-size"),
				SkipUsers:     c.Bool("skip-users"),
				SkipTracks:    c.Bool("skip-tracks"),
				SkipPlaylists: c.Bool("skip-playlists"),
				SkipSocial:    c.Bool("skip-social"),
				SkipPlays:     c.Bool("skip-plays"),
			}

			r, err := NewReplayer(cfg, logger)
			if err != nil {
				return fmt.Errorf("init replayer: %w", err)
			}
			defer r.Close()

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			return r.Run(ctx)
		},
	}
}

func parsePrivKey(s string) (*ecdsa.PrivateKey, error) {
	s = strings.TrimPrefix(s, "0x")
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return crypto.ToECDSA(b)
}
