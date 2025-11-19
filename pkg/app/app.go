package app

import (
	"context"

	"github.com/OpenAudio/go-openaudio/pkg/common"
	"github.com/OpenAudio/go-openaudio/pkg/config"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type App struct {
	// core      v1connect.CoreHandler
	// storage   v1connect.StorageHandler
	// ddex      v1connect.DDEXHandler
	// validator v1connect.ValidatorHandler
	// p2p       v1connect.P2PHandler
	// system    v1connect.SystemHandler
}

func NewApp(config *config.Config) *App {
	return &App{}
}

// run starts the app and blocks until the app is stopped
func (app *App) Run(ctx context.Context) error {
	// TODO: create service implementations for each service

	// TODO: dependency inject the services into the app and each other

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger(), middleware.Recover(), common.InjectRealIP())

	rpcGroup := e.Group("")
	rpcGroup.Use(common.CORS())

	// TODO: add connect handlers for each service impl

	// TODO: register console routes at root

	// TODO: register GET stream routes

	// TODO: run all services in errgroup

	// TODO: wait for ctx to be done

	// TODO: shutdown all services

	// TODO: return any errors
	return ctx.Err()
}
