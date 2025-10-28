package server

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/storage/v1"
	"github.com/OpenAudio/go-openaudio/pkg/api/storage/v1/v1connect"
)

var _ v1connect.StorageServiceHandler = (*StorageServer)(nil)

type StorageServer struct {
}

// GetHealth implements v1connect.StorageServiceHandler.
func (s *StorageServer) GetHealth(context.Context, *connect.Request[v1.GetHealthRequest]) (*connect.Response[v1.GetHealthResponse], error) {
	panic("unimplemented")
}

// GetIPData implements v1connect.StorageServiceHandler.
func (s *StorageServer) GetIPData(context.Context, *connect.Request[v1.GetIPDataRequest]) (*connect.Response[v1.GetIPDataResponse], error) {
	panic("unimplemented")
}

// GetRendezvousNodes implements v1connect.StorageServiceHandler.
func (s *StorageServer) GetRendezvousNodes(context.Context, *connect.Request[v1.GetRendezvousNodesRequest]) (*connect.Response[v1.GetRendezvousNodesResponse], error) {
	panic("unimplemented")
}

// GetStatus implements v1connect.StorageServiceHandler.
func (s *StorageServer) GetStatus(context.Context, *connect.Request[v1.GetStatusRequest]) (*connect.Response[v1.GetStatusResponse], error) {
	panic("unimplemented")
}

// GetStreamURL implements v1connect.StorageServiceHandler.
func (s *StorageServer) GetStreamURL(context.Context, *connect.Request[v1.GetStreamURLRequest]) (*connect.Response[v1.GetStreamURLResponse], error) {
	panic("unimplemented")
}

// GetUpload implements v1connect.StorageServiceHandler.
func (s *StorageServer) GetUpload(context.Context, *connect.Request[v1.GetUploadRequest]) (*connect.Response[v1.GetUploadResponse], error) {
	panic("unimplemented")
}

// Ping implements v1connect.StorageServiceHandler.
func (s *StorageServer) Ping(context.Context, *connect.Request[v1.PingRequest]) (*connect.Response[v1.PingResponse], error) {
	return connect.NewResponse(&v1.PingResponse{
		Message: "pong",
	}), nil
}

// StreamTrack implements v1connect.StorageServiceHandler.
func (s *StorageServer) StreamTrack(context.Context, *connect.Request[v1.StreamTrackRequest], *connect.ServerStream[v1.StreamTrackResponse]) error {
	panic("unimplemented")
}

// UploadFiles implements v1connect.StorageServiceHandler.
func (s *StorageServer) UploadFiles(context.Context, *connect.Request[v1.UploadFilesRequest]) (*connect.Response[v1.UploadFilesResponse], error) {
	panic("unimplemented")
}
