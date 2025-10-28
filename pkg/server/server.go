package server

import (
	"context"

	"github.com/OpenAudio/go-openaudio/pkg/config"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type Server struct {
	ctx    context.Context
	config *config.Config
	logger *zap.Logger
	e      *echo.Echo

	core    *CoreServer
	storage *StorageServer
	system  *SystemServer
	eth     *EthServer
}

func NewServer(ctx context.Context, config *config.Config, logger *zap.Logger) *Server {
	return &Server{
		ctx:    ctx,
		config: config,
		logger: logger,
		e:      echo.New(),
	}
}

func (s *Server) Init() error {
	ecfg := s.config.OpenAudio.Server.Echo

	e := s.e

	e.HideBanner = true
	e.Logger = (*ZapEchoLogger)(s.logger)

	e.Use(middleware.CORS())
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())

	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(rate.Limit(ecfg.IPRateLimit))))
	e.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
		Timeout: ecfg.RequestTimeout,
	}))

	s.RegisterRoutes(e)

	return nil
}
