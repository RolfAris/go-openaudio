// contains all the routes that core serves
package server

import (
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/OpenAudio/go-openaudio/pkg/core/console/views/sandbox"
	"github.com/labstack/echo/v4"
)

// RegisterRoutes registers all core HTTP routes on the provided echo instance
func (s *Server) RegisterRoutes(e *echo.Echo) error {
	s.logger.Info("registering core HTTP routes")

	g := e.Group("/core")

	// TODO: add into connectrpc
	g.GET("/rewards", s.getRewards)
	g.GET("/rewards/attestation", s.getRewardAttestation)
	g.GET("/nodes", s.getRegisteredNodes)
	g.GET("/nodes/verbose", s.getRegisteredNodes)
	g.GET("/nodes/discovery", s.getRegisteredNodes)
	g.GET("/nodes/discovery/verbose", s.getRegisteredNodes)
	g.GET("/nodes/content", s.getRegisteredNodes)
	g.GET("/nodes/content/verbose", s.getRegisteredNodes)
	g.GET("/status", func(c echo.Context) error {
		if s.self == nil {
			return c.String(http.StatusServiceUnavailable, "starting up")
		}
		res, err := s.self.GetStatus(c.Request().Context(), &connect.Request[v1.GetStatusRequest]{})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, res.Msg)
	})

	g.Any("/crpc*", s.proxyCometRequest)

	// Register Eth RPC routes
	if err := s.registerEthRPC(e); err != nil {
		return fmt.Errorf("failed to register eth rpc: %w", err)
	}

	g.GET("/sdk", echo.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sandbox.ServeSandbox(s.config, w, r)
	})))

	return nil
}
