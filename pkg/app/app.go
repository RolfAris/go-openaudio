package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/OpenAudio/go-openaudio/pkg/common"
	"github.com/OpenAudio/go-openaudio/pkg/config"
	coreServer "github.com/OpenAudio/go-openaudio/pkg/core/server"
	"github.com/OpenAudio/go-openaudio/pkg/eth"
	storageServer "github.com/OpenAudio/go-openaudio/pkg/mediorum/server"
	"github.com/OpenAudio/go-openaudio/pkg/system"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	corev1connect "github.com/OpenAudio/go-openaudio/pkg/api/core/v1/v1connect"
	ethv1connect "github.com/OpenAudio/go-openaudio/pkg/api/eth/v1/v1connect"
	storagev1connect "github.com/OpenAudio/go-openaudio/pkg/api/storage/v1/v1connect"
	systemv1connect "github.com/OpenAudio/go-openaudio/pkg/api/system/v1/v1connect"
)

type App struct {
	config *config.Config
	logger *zap.Logger

	// Server instances
	httpServer   *echo.Echo // HTTP/HTTPS REST endpoints
	grpcServer   *echo.Echo // gRPC h2c (cleartext)
	grpcsServer  *echo.Echo // gRPC TLS (encrypted)
	socketServer *echo.Echo // Unix socket

	// Services
	ethService     *eth.EthService
	coreService    *coreServer.CoreService
	storageService *storageServer.StorageService
	systemService  *system.SystemService
}

func NewApp(config *config.Config, logger *zap.Logger) *App {
	// create services
	ethService := eth.NewEthService(config, logger)
	coreService := coreServer.NewCoreService()
	storageService := storageServer.NewStorageService()
	systemService := system.NewSystemService()

	// wire up dependencies
	coreService.SetStorageService(storageService)
	coreService.SetEthService(ethService)

	storageService.SetCoreService(coreService)
	storageService.SetEthService(ethService)

	systemService.SetCoreService(coreService)
	systemService.SetStorageService(storageService)

	app := &App{
		config:         config,
		logger:         logger,
		httpServer:     echo.New(),
		grpcServer:     echo.New(),
		grpcsServer:    echo.New(),
		socketServer:   echo.New(),
		ethService:     ethService,
		coreService:    coreService,
		storageService: storageService,
		systemService:  systemService,
	}

	return app
}

// run starts the app and blocks until the app is stopped
func (app *App) Run(ctx context.Context) error {
	app.logger.Info("good morning!")

	// Register routes for all server instances
	app.registerRoutes(app.httpServer)
	app.registerRoutes(app.grpcServer)
	app.registerRoutes(app.grpcsServer)
	app.registerRoutes(app.socketServer)

	eg, ctx := errgroup.WithContext(ctx)

	// Shutdown handler
	eg.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return app.Shutdown(shutdownCtx)
	})

	// Start the server(s)
	eg.Go(func() error {
		return app.runServer(ctx)
	})

	// Log sync
	eg.Go(func() error {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				app.logger.Sync()
			case <-ctx.Done():
				return nil
			}
		}
	})

	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

func (app *App) registerRoutes(e *echo.Echo) {
	e.HideBanner = true
	e.Use(middleware.Logger(), middleware.Recover(), common.InjectRealIP())

	rpcGroup := e.Group("")
	rpcGroup.Use(common.CORS())

	corePath, coreHandler := corev1connect.NewCoreServiceHandler(app.coreService, connect.WithInterceptors(coreServer.ReadyCheckInterceptor(app.coreService)))
	rpcGroup.POST(corePath+"*", echo.WrapHandler(coreHandler))

	storagePath, storageHandler := storagev1connect.NewStorageServiceHandler(app.storageService)
	rpcGroup.POST(storagePath+"*", echo.WrapHandler(storageHandler))

	systemPath, systemHandler := systemv1connect.NewSystemServiceHandler(app.systemService)
	rpcGroup.POST(systemPath+"*", echo.WrapHandler(systemHandler))

	ethPath, ethHandler := ethv1connect.NewEthServiceHandler(app.ethService)
	rpcGroup.POST(ethPath+"*", echo.WrapHandler(ethHandler))

	// TODO: register console routes at root

	e.GET("/up", func(c echo.Context) error {
		return c.String(http.StatusOK, "up")
	})

	e.GET("/ready", func(c echo.Context) error {
		return c.String(http.StatusOK, "ready")
	})

	// TODO: register GET stream routes
}

func (app *App) Shutdown(ctx context.Context) error {
	var errs []error

	// Shutdown HTTP server
	if app.httpServer != nil {
		if err := app.httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown http server: %w", err))
		}
	}

	// Shutdown gRPC h2c server
	if app.grpcServer != nil {
		if err := app.grpcServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown grpc server: %w", err))
		}
	}

	// Shutdown gRPC TLS server
	if app.grpcsServer != nil {
		if err := app.grpcsServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown grpcs server: %w", err))
		}
	}

	// Shutdown socket server
	if app.socketServer != nil {
		if err := app.socketServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown socket server: %w", err))
		}
	}

	// TODO: shut down all services
	app.logger.Info("goodnight!")
	return errors.Join(errs...)
}
