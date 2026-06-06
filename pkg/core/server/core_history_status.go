package server

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (s *Server) getCoreHistoryStatus(c echo.Context) error {
	if s.db == nil {
		return c.String(http.StatusServiceUnavailable, "database not ready")
	}

	status, err := s.db.GetCoreHistoryStatus(c.Request().Context(), s.coreHistoryRetainFloor())
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, status)
}

func (s *Server) coreHistoryRetainFloor() int64 {
	if s == nil || s.config == nil || s.config.Archive {
		return 0
	}

	if s.abciState != nil && s.abciState.lastRetainHeight > 0 {
		return s.abciState.lastRetainHeight
	}

	if s.cache == nil {
		return 0
	}

	currentHeight := s.cache.currentHeight.Load()
	if currentHeight <= s.config.RetainHeight {
		return 0
	}

	return currentHeight - s.config.RetainHeight
}
