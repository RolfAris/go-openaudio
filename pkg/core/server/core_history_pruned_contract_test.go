package server

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/OpenAudio/go-openaudio/pkg/core/config"
	"github.com/stretchr/testify/require"
)

func TestCoreHistoryIsBelowRetainFloorUsesLastRetainHeight(t *testing.T) {
	s := &Server{
		config: &config.Config{RetainHeight: 100},
		cache:  &Cache{},
		abciState: &ABCIState{
			lastRetainHeight: 900,
		},
	}
	s.cache.currentHeight.Store(1_000)

	retainFloor, belowFloor := s.coreHistoryIsBelowRetainFloor(899)
	require.Equal(t, int64(900), retainFloor)
	require.True(t, belowFloor)

	_, belowFloor = s.coreHistoryIsBelowRetainFloor(900)
	require.False(t, belowFloor)

	_, belowFloor = s.coreHistoryIsBelowRetainFloor(901)
	require.False(t, belowFloor)
}

func TestCoreHistoryIsBelowRetainFloorFallsBackToCurrentHeightMinusWindow(t *testing.T) {
	s := &Server{
		config:    &config.Config{RetainHeight: 100},
		cache:     &Cache{},
		abciState: &ABCIState{},
	}
	s.cache.currentHeight.Store(1_000)

	retainFloor, belowFloor := s.coreHistoryIsBelowRetainFloor(899)
	require.Equal(t, int64(900), retainFloor)
	require.True(t, belowFloor)
}

func TestCoreHistoryIsBelowRetainFloorDisabledForArchiveNodes(t *testing.T) {
	s := &Server{
		config: &config.Config{
			Archive:      true,
			RetainHeight: 100,
		},
		cache: &Cache{},
	}
	s.cache.currentHeight.Store(1_000)

	retainFloor, belowFloor := s.coreHistoryIsBelowRetainFloor(1)
	require.Zero(t, retainFloor)
	require.False(t, belowFloor)
}

func TestCoreHistoryPrunedErrorReturnsNotFoundBelowRetainFloor(t *testing.T) {
	s := &Server{
		config:    &config.Config{RetainHeight: 100},
		cache:     &Cache{},
		abciState: &ABCIState{},
	}
	s.cache.currentHeight.Store(1_000)
	c := &CoreService{core: s}

	err := c.coreHistoryPrunedError("block", 899)

	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.ErrorContains(t, err, "block at height 899 is below retained core history floor 900")
}

func TestCoreHistoryPrunedErrorReturnsNilAtOrAboveRetainFloor(t *testing.T) {
	s := &Server{
		config:    &config.Config{RetainHeight: 100},
		cache:     &Cache{},
		abciState: &ABCIState{},
	}
	s.cache.currentHeight.Store(1_000)
	c := &CoreService{core: s}

	require.NoError(t, c.coreHistoryPrunedError("block", 900))
	require.NoError(t, c.coreHistoryPrunedError("block", 901))
}
