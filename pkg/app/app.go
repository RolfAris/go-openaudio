package app

import (
	"context"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type App struct {
	ctx    context.Context
	logger *zap.Logger
}

func NewApp(ctx context.Context, logger *zap.Logger) *App {
	// core := core.NewCore(ctx, logger)
	// storage := storage.NewStorage(ctx, logger)

	// core.SetStorage(storage)
	// storage.SetCore(core)

	// server := server.NewServer(ctx, logger, core, storage)

	return &App{
		ctx:    ctx,
		logger: logger,
	}
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
		return nil // app.server.Run()
	})

	g.Go(func() error {
		<-app.ctx.Done()
		return app.ctx.Err()
	})

	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}
