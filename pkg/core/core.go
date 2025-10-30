package core

import (
	"context"
	"fmt"
	_ "net/http/pprof"

	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/OpenAudio/go-openaudio/pkg/core/config"
	"github.com/OpenAudio/go-openaudio/pkg/core/db"
	"github.com/OpenAudio/go-openaudio/pkg/core/server"
	"github.com/OpenAudio/go-openaudio/pkg/eth"
	"github.com/OpenAudio/go-openaudio/pkg/lifecycle"
	"github.com/OpenAudio/go-openaudio/pkg/pos"
	"github.com/OpenAudio/go-openaudio/pkg/types"
	cconfig "github.com/cometbft/cometbft/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ types.CoreService = (*Core)(nil)

type Core struct {
	logger *zap.Logger

	storage types.StorageService
}

func NewCore(ctx context.Context, z *zap.Logger) *Core {
	l := z.With(zap.String("service", "core"))
	return &Core{
		logger: l,
	}
}

// SetStorage wires up the storage service dependency
func (c *Core) SetStorage(s types.StorageService) {
	c.storage = s
}

// GetBlock implements CoreService.
func (c *Core) GetBlock(ctx context.Context) (*v1.Block, error) {
	return &v1.Block{
		Height:    -1,
		Hash:      "blockhash",
		Proposer:  "me",
		ChainId:   "OAP",
		Timestamp: timestamppb.Now(),
	}, nil
}

// InitResult holds the results of core initialization
type InitResult struct {
	Server      *server.Server
	Config      *config.Config
	CometConfig *cconfig.Config
	Pool        *pgxpool.Pool
}

// InitCoreServer initializes and returns the core server without starting it
func InitCoreServer(ctx context.Context, lc *lifecycle.Lifecycle, logger *zap.Logger, posChannel chan pos.PoSRequest, ethService *eth.EthService) (*InitResult, error) {
	logger.Info("initializing core server")

	coreConfig, cometConfig, err := config.SetupNode(logger)
	if err != nil {
		return nil, fmt.Errorf("setting up node: %v", err)
	}

	logger.Info("configuration created")

	// db migrations
	if err := db.RunMigrations(logger, coreConfig.PSQLConn, coreConfig.RunDownMigrations()); err != nil {
		return nil, fmt.Errorf("running migrations: %v", err)
	}

	logger.Info("db migrations successful")

	// Use the passed context for the pool
	pool, err := pgxpool.New(ctx, coreConfig.PSQLConn)
	if err != nil {
		return nil, fmt.Errorf("couldn't create pgx pool: %v", err)
	}

	s := server.NewServer(lc, coreConfig, cometConfig, logger, pool, ethService, posChannel)

	if err := s.Init(); err != nil {
		return nil, fmt.Errorf("initializing core server: %v", err)
	}

	logger.Info("core server initialized")

	return &InitResult{
		Server:      s,
		Config:      coreConfig,
		CometConfig: cometConfig,
		Pool:        pool,
	}, nil
}

// Run is the legacy entry point that handles full core lifecycle (deprecated, use InitCoreServer + Start instead)
func Run(ctx context.Context, lc *lifecycle.Lifecycle, logger *zap.Logger, posChannel chan pos.PoSRequest, coreService *server.CoreService, ethService *eth.EthService) error {
	result, err := InitCoreServer(ctx, lc, logger, posChannel, ethService)
	if err != nil {
		return err
	}
	defer result.Pool.Close()

	// Wire core service
	coreService.SetCore(result.Server)

	// Note: Console registration moved to app layer
	// TODO: Remove this function once app.go is fully updated

	if err := result.Server.Start(); err != nil {
		logger.Error("core service crashed", zap.Error(err))
		return err
	}

	return result.Server.Shutdown()
}
