package server

import (
	"fmt"

	"connectrpc.com/connect"
)

func (s *Server) coreHistoryIsBelowRetainFloor(height int64) (int64, bool) {
	if height <= 0 {
		return 0, false
	}

	retainFloor := s.coreHistoryRetainFloor()
	if retainFloor <= 0 {
		return retainFloor, false
	}

	return retainFloor, height < retainFloor
}

func (c *CoreService) coreHistoryPrunedError(kind string, height int64) error {
	if c == nil || c.core == nil {
		return nil
	}

	retainFloor, belowFloor := c.core.coreHistoryIsBelowRetainFloor(height)
	if !belowFloor {
		return nil
	}

	return connect.NewError(
		connect.CodeNotFound,
		fmt.Errorf("%s at height %d is below retained core history floor %d", kind, height, retainFloor),
	)
}
