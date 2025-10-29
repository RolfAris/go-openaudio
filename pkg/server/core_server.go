package server

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/OpenAudio/go-openaudio/pkg/api/core/v1/v1connect"
	"github.com/OpenAudio/go-openaudio/pkg/types"
)

var _ v1connect.CoreServiceHandler = (*CoreServer)(nil)

type CoreServer struct {
	core types.CoreService
}

func NewCoreServer() *CoreServer {
	return &CoreServer{}
}

// SetCore wires up the actual core service implementation
func (cs *CoreServer) SetCore(c types.CoreService) {
	cs.core = c
}

// ForwardTransaction implements v1connect.CoreServiceHandler.
func (c *CoreServer) ForwardTransaction(context.Context, *connect.Request[v1.ForwardTransactionRequest]) (*connect.Response[v1.ForwardTransactionResponse], error) {
	panic("unimplemented")
}

// GetBlock implements v1connect.CoreServiceHandler.
func (cs *CoreServer) GetBlock(ctx context.Context, req *connect.Request[v1.GetBlockRequest]) (*connect.Response[v1.GetBlockResponse], error) {
	if cs.core == nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("core service not initialized"))
	}
	
	block, err := cs.core.GetBlock(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	
	return connect.NewResponse(&v1.GetBlockResponse{
		Block: block,
	}), nil
}

// GetBlocks implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetBlocks(context.Context, *connect.Request[v1.GetBlocksRequest]) (*connect.Response[v1.GetBlocksResponse], error) {
	panic("unimplemented")
}

// GetDeregistrationAttestation implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetDeregistrationAttestation(context.Context, *connect.Request[v1.GetDeregistrationAttestationRequest]) (*connect.Response[v1.GetDeregistrationAttestationResponse], error) {
	panic("unimplemented")
}

// GetERN implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetERN(context.Context, *connect.Request[v1.GetERNRequest]) (*connect.Response[v1.GetERNResponse], error) {
	panic("unimplemented")
}

// GetHealth implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetHealth(context.Context, *connect.Request[v1.GetHealthRequest]) (*connect.Response[v1.GetHealthResponse], error) {
	panic("unimplemented")
}

// GetMEAD implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetMEAD(context.Context, *connect.Request[v1.GetMEADRequest]) (*connect.Response[v1.GetMEADResponse], error) {
	panic("unimplemented")
}

// GetNodeInfo implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetNodeInfo(context.Context, *connect.Request[v1.GetNodeInfoRequest]) (*connect.Response[v1.GetNodeInfoResponse], error) {
	panic("unimplemented")
}

// GetPIE implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetPIE(context.Context, *connect.Request[v1.GetPIERequest]) (*connect.Response[v1.GetPIEResponse], error) {
	panic("unimplemented")
}

// GetRegistrationAttestation implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetRegistrationAttestation(context.Context, *connect.Request[v1.GetRegistrationAttestationRequest]) (*connect.Response[v1.GetRegistrationAttestationResponse], error) {
	panic("unimplemented")
}

// GetReward implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetReward(context.Context, *connect.Request[v1.GetRewardRequest]) (*connect.Response[v1.GetRewardResponse], error) {
	panic("unimplemented")
}

// GetRewardAttestation implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetRewardAttestation(context.Context, *connect.Request[v1.GetRewardAttestationRequest]) (*connect.Response[v1.GetRewardAttestationResponse], error) {
	panic("unimplemented")
}

// GetRewards implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetRewards(context.Context, *connect.Request[v1.GetRewardsRequest]) (*connect.Response[v1.GetRewardsResponse], error) {
	panic("unimplemented")
}

// GetSlashAttestation implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetSlashAttestation(context.Context, *connect.Request[v1.GetSlashAttestationRequest]) (*connect.Response[v1.GetSlashAttestationResponse], error) {
	panic("unimplemented")
}

// GetSlashAttestations implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetSlashAttestations(context.Context, *connect.Request[v1.GetSlashAttestationsRequest]) (*connect.Response[v1.GetSlashAttestationsResponse], error) {
	panic("unimplemented")
}

// GetStatus implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetStatus(context.Context, *connect.Request[v1.GetStatusRequest]) (*connect.Response[v1.GetStatusResponse], error) {
	panic("unimplemented")
}

// GetStoredSnapshots implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetStoredSnapshots(context.Context, *connect.Request[v1.GetStoredSnapshotsRequest]) (*connect.Response[v1.GetStoredSnapshotsResponse], error) {
	panic("unimplemented")
}

// GetStreamURLs implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetStreamURLs(context.Context, *connect.Request[v1.GetStreamURLsRequest]) (*connect.Response[v1.GetStreamURLsResponse], error) {
	panic("unimplemented")
}

// GetTransaction implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetTransaction(context.Context, *connect.Request[v1.GetTransactionRequest]) (*connect.Response[v1.GetTransactionResponse], error) {
	panic("unimplemented")
}

// GetUploadByCID implements v1connect.CoreServiceHandler.
func (c *CoreServer) GetUploadByCID(context.Context, *connect.Request[v1.GetUploadByCIDRequest]) (*connect.Response[v1.GetUploadByCIDResponse], error) {
	panic("unimplemented")
}

// Ping implements v1connect.CoreServiceHandler.
func (c *CoreServer) Ping(_ context.Context, req *connect.Request[v1.PingRequest]) (*connect.Response[v1.PingResponse], error) {
	return connect.NewResponse(&v1.PingResponse{
		Message: "pong",
	}), nil
}

// SendTransaction implements v1connect.CoreServiceHandler.
func (c *CoreServer) SendTransaction(context.Context, *connect.Request[v1.SendTransactionRequest]) (*connect.Response[v1.SendTransactionResponse], error) {
	panic("unimplemented")
}
