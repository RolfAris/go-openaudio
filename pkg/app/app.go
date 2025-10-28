package app

import (
	"context"
	"fmt"

	"github.com/OpenAudio/go-openaudio/pkg/config"
	"github.com/OpenAudio/go-openaudio/pkg/server"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type App struct {
	ctx    context.Context
	config *config.Config
	logger *zap.Logger

	server *server.Server
}

func NewApp(ctx context.Context, config *config.Config) *App {
	// core := core.NewCore(ctx, logger)
	// storage := storage.NewStorage(ctx, logger)

	// core.SetStorage(storage)
	// storage.SetCore(core)

	return &App{
		ctx:    ctx,
		config: config,
	}
}

func (app *App) Init() error {
	logger, err := app.config.OpenAudio.Logger.Build()
	if err != nil {
		return fmt.Errorf("building logger: %w", err)
	}
	app.logger = logger

	app.server = server.NewServer(app.ctx, app.config, app.logger)
	if err := app.server.Init(); err != nil {
		return fmt.Errorf("initializing server: %w", err)
	}

	return nil
}

func (app *App) Run() error {
	app.logger.Info("good morning!")
	defer app.logger.Info("goodnight...")

	var g errgroup.Group

	g.Go(func() error {
		return nil // app.core.Run()
	})

	g.Go(func() error {
		return nil // app.storage.Run()
	})

	g.Go(func() error {
		if err := app.server.Run(); err != nil {
			return fmt.Errorf("server crashed: %w", err)
		}
		app.logger.Info("server shutdown")
		return nil
	})

	g.Go(func() error {
		<-app.ctx.Done()
		app.logger.Info("app shutdown")
		// Context cancellation during shutdown is expected, not an error
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}
