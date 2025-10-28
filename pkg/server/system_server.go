package server

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/system/v1"
	"github.com/OpenAudio/go-openaudio/pkg/api/system/v1/v1connect"
)

var _ v1connect.SystemServiceHandler = (*SystemServer)(nil)

type SystemServer struct{}

func NewSystemServer() *SystemServer {
	return &SystemServer{}
}

// GetHealth implements v1connect.SystemServiceHandler.
func (s *SystemServer) GetHealth(context.Context, *connect.Request[v1.GetHealthRequest]) (*connect.Response[v1.GetHealthResponse], error) {
	panic("unimplemented")
}

// Ping implements v1connect.SystemServiceHandler.
func (s *SystemServer) Ping(context.Context, *connect.Request[v1.PingRequest]) (*connect.Response[v1.PingResponse], error) {
	return connect.NewResponse(&v1.PingResponse{
		Message: "pong",
	}), nil
}
