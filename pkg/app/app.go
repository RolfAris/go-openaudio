package app

import (
	"context"
	"fmt"

	"github.com/OpenAudio/go-openaudio/pkg/config"
	"github.com/OpenAudio/go-openaudio/pkg/core"
	coreConfig "github.com/OpenAudio/go-openaudio/pkg/core/config"
	coreServer "github.com/OpenAudio/go-openaudio/pkg/core/server"
	"github.com/OpenAudio/go-openaudio/pkg/eth"
	"github.com/OpenAudio/go-openaudio/pkg/lifecycle"
	"github.com/OpenAudio/go-openaudio/pkg/mediorum"
	mediorumServer "github.com/OpenAudio/go-openaudio/pkg/mediorum/server"
	"github.com/OpenAudio/go-openaudio/pkg/pos"
	"github.com/OpenAudio/go-openaudio/pkg/server"
	"github.com/OpenAudio/go-openaudio/pkg/system"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type App struct {
	ctx    context.Context
	config *config.Config
	logger *zap.Logger

	// Service layer (implements interfaces)
	coreService    *core.Core
	storageService *mediorum.Storage

	// RPC/HTTP server
	server *server.Server

	// Infrastructure
	lc                *lifecycle.Lifecycle
	posChannel        chan pos.PoSRequest
	ethService        *eth.EthService
	coreRPCService    *coreServer.CoreService
	storageRPCService *mediorumServer.StorageService
}

func NewApp(ctx context.Context, config *config.Config) *App {
	return &App{
		ctx:    ctx,
		config: config,
	}
}

func (app *App) Init() error {
	// 1. Build logger
	logger, err := app.config.OpenAudio.Logger.Build()
	if err != nil {
		return fmt.Errorf("building logger: %w", err)
	}
	app.logger = logger

	// 2. Create lifecycle and infrastructure
	app.lc = lifecycle.NewLifecycle(app.ctx, "app", logger)
	app.posChannel = make(chan pos.PoSRequest, 100)

	// 3. Create service layer (lightweight, just interfaces)
	app.coreService = core.NewCore(app.ctx, logger)
	app.storageService = mediorum.NewStorage(app.ctx, logger)

	// 4. Wire circular dependencies
	app.coreService.SetStorage(app.storageService)
	app.storageService.SetCore(app.coreService)

	// 5. Create actual RPC service implementations (existing handlers)
	// These are the REAL service implementations that exist
	dbUrl := coreConfig.GetDbURL()
	app.ethService = eth.NewEthService(
		dbUrl,
		coreConfig.GetEthRPC(),
		coreConfig.GetRegistryAddress(),
		app.logger,
		coreConfig.GetRuntimeEnvironment(),
	)

	app.coreRPCService = coreServer.NewCoreService()
	app.storageRPCService = mediorumServer.NewStorageService()

	// Only set storage service on core if storage is enabled
	// TODO: Make this configurable
	app.coreRPCService.SetStorageService(app.storageRPCService)

	systemService := system.NewSystemService(app.coreRPCService, app.storageRPCService)

	// 6. Initialize HTTP/RPC server with real service implementations
	app.server = server.NewServer(
		app.ctx,
		app.config,
		app.logger,
		app.coreRPCService,
		app.storageRPCService,
		systemService,
		app.ethService,
	)

	if err := app.server.Init(); err != nil {
		return fmt.Errorf("initializing server: %w", err)
	}

	return nil
}

func (app *App) Run() error {
	app.logger.Info("good morning!")
	defer app.logger.Info("goodnight...")

	var g errgroup.Group

	// Run Core (blockchain, consensus, CometBFT)
	g.Go(func() error {
		if err := core.Run(
			app.ctx,
			app.lc,
			app.logger,
			app.posChannel,
			app.coreRPCService,
			app.ethService,
		); err != nil {
			return fmt.Errorf("core crashed: %w", err)
		}
		return nil
	})

	// Run Storage (mediorum, file storage, content delivery)
	g.Go(func() error {
		if err := mediorum.Run(
			app.lc,
			app.logger,
			app.posChannel,
			app.storageRPCService,
			app.coreRPCService,
		); err != nil {
			return fmt.Errorf("storage crashed: %w", err)
		}
		return nil
	})

	// Run Eth Service
	g.Go(func() error {
		if err := app.ethService.Run(app.ctx); err != nil {
			return fmt.Errorf("eth service crashed: %w", err)
		}
		return nil
	})

	// Run HTTP/RPC Server
	g.Go(func() error {
		if err := app.server.Run(); err != nil {
			return fmt.Errorf("server crashed: %w", err)
		}
		app.logger.Info("server shutdown")
		return nil
	})

	// Wait for context cancellation (graceful shutdown)
	g.Go(func() error {
		<-app.ctx.Done()
		app.logger.Info("app shutdown signal received")
		// Context cancellation is expected, not an error
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}
