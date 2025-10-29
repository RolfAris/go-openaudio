package server

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/storage/v1"
	"github.com/OpenAudio/go-openaudio/pkg/api/storage/v1/v1connect"
	"github.com/OpenAudio/go-openaudio/pkg/types"
)

var _ v1connect.StorageServiceHandler = (*StorageServer)(nil)

type StorageServer struct {
	storage types.StorageService
}

func NewStorageServer() *StorageServer {
	return &StorageServer{}
}

// SetStorage wires up the actual storage service implementation
func (ss *StorageServer) SetStorage(s types.StorageService) {
	ss.storage = s
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
func (ss *StorageServer) GetUpload(ctx context.Context, req *connect.Request[v1.GetUploadRequest]) (*connect.Response[v1.GetUploadResponse], error) {
	if ss.storage == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("storage service not initialized"))
	}
	
	uploadStr, err := ss.storage.GetUpload(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	
	// TODO: Proper conversion from service layer to proto
	// For now, just return a basic upload response with the string data
	return connect.NewResponse(&v1.GetUploadResponse{
		Upload: &v1.Upload{
			// Populate fields from uploadStr when we have proper types
			Id: uploadStr,
		},
	}), nil
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
