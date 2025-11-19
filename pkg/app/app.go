package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/OpenAudio/go-openaudio/pkg/common"
	"github.com/OpenAudio/go-openaudio/pkg/config"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type App struct {
	e      *echo.Echo
	logger *zap.Logger
	// core      v1connect.CoreHandler
	// storage   v1connect.StorageHandler
	// ddex      v1connect.DDEXHandler
	// validator v1connect.ValidatorHandler
	// p2p       v1connect.P2PHandler
	// system    v1connect.SystemHandler
}

func NewApp(config *config.Config, logger *zap.Logger) *App {
	return &App{
		e:      echo.New(),
		logger: logger,
	}
}

// run starts the app and blocks until the app is stopped
func (app *App) Run(ctx context.Context) error {
	app.logger.Info("good morning!")
	// TODO: create service implementations for each service

	// TODO: dependency inject the services into the app and each other

	app.e.HideBanner = true
	app.e.Use(middleware.Logger(), middleware.Recover(), common.InjectRealIP())

	rpcGroup := app.e.Group("")
	rpcGroup.Use(common.CORS())

	// TODO: add connect handlers for each service impl

	// TODO: register console routes at root

	app.e.GET("/up", func(c echo.Context) error {
		return c.String(http.StatusOK, "up")
	})

	app.e.GET("/ready", func(c echo.Context) error {
		return c.String(http.StatusOK, "ready")
	})

	// TODO: register GET stream routes

	// TODO: run all services in errgroup

	// TODO: wait for ctx to be done

	// TODO: shutdown all services

	// TODO: return any errors

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return app.Shutdown(shutdownCtx)
	})

	eg.Go(func() error {
		if err := app.e.Start(":8080"); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("start echo server: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		// sync logs
		ticker := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-ticker.C:
				app.logger.Sync()
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

func (app *App) Shutdown(ctx context.Context) error {
	var errs []error
	// TODO: shut down echo server
	if err := app.e.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("shutdown echo server: %w", err))
	}

	// TODO: shut down all services
	app.logger.Info("goodnight!")
	return errors.Join(errs...)
}
