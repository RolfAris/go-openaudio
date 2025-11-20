package core

import (
	"context"

	"github.com/OpenAudio/go-openaudio/pkg/config"
	"github.com/OpenAudio/go-openaudio/store"
	"go.uber.org/zap"
)

type Core struct {
	config *config.Config
	logger *zap.Logger
	store  *store.Store
}

func NewCore(config *config.Config, logger *zap.Logger, store *store.Store) *Core {
	return &Core{
		config: config,
		logger: logger,
		store:  store,
	}
}

func (c *Core) Run(ctx context.Context) error {
	return nil
}

func (c *Core) Shutdown(ctx context.Context) error {
	return nil
}
