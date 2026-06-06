package server

import (
	"testing"

	"github.com/OpenAudio/go-openaudio/pkg/core/config"
	"github.com/stretchr/testify/require"
)

func TestCoreHistoryRetainFloorUsesLastSetRetainHeight(t *testing.T) {
	cfg := &config.Config{RetainHeight: 100}
	server := &Server{
		config:    cfg,
		cache:     &Cache{},
		abciState: NewABCIState(900),
	}
	server.cache.currentHeight.Store(1200)

	require.EqualValues(t, 900, server.coreHistoryRetainFloor())
}

func TestCoreHistoryRetainFloorFallsBackToTargetWindow(t *testing.T) {
	cfg := &config.Config{RetainHeight: 100}
	server := &Server{
		config:    cfg,
		cache:     &Cache{},
		abciState: NewABCIState(0),
	}
	server.cache.currentHeight.Store(1200)

	require.EqualValues(t, 1100, server.coreHistoryRetainFloor())
}

func TestCoreHistoryRetainFloorDisabledForArchiveMode(t *testing.T) {
	cfg := &config.Config{Archive: true, RetainHeight: 100}
	server := &Server{
		config:    cfg,
		cache:     &Cache{},
		abciState: NewABCIState(900),
	}
	server.cache.currentHeight.Store(1200)

	require.EqualValues(t, 0, server.coreHistoryRetainFloor())
}
