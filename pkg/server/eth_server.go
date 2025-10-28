package server

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/eth/v1"
	"github.com/OpenAudio/go-openaudio/pkg/api/eth/v1/v1connect"
)

var _ v1connect.EthServiceHandler = (*EthServer)(nil)

type EthServer struct{}

// GetActiveSlashProposalForAddress implements v1connect.EthServiceHandler.
func (e *EthServer) GetActiveSlashProposalForAddress(context.Context, *connect.Request[v1.GetActiveSlashProposalForAddressRequest]) (*connect.Response[v1.GetActiveSlashProposalForAddressResponse], error) {
	panic("unimplemented")
}

// GetLatestFundingRound implements v1connect.EthServiceHandler.
func (e *EthServer) GetLatestFundingRound(context.Context, *connect.Request[v1.GetLatestFundingRoundRequest]) (*connect.Response[v1.GetLatestFundingRoundResponse], error) {
	panic("unimplemented")
}

// GetRegisteredEndpointInfo implements v1connect.EthServiceHandler.
func (e *EthServer) GetRegisteredEndpointInfo(context.Context, *connect.Request[v1.GetRegisteredEndpointInfoRequest]) (*connect.Response[v1.GetRegisteredEndpointInfoResponse], error) {
	panic("unimplemented")
}

// GetRegisteredEndpoints implements v1connect.EthServiceHandler.
func (e *EthServer) GetRegisteredEndpoints(context.Context, *connect.Request[v1.GetRegisteredEndpointsRequest]) (*connect.Response[v1.GetRegisteredEndpointsResponse], error) {
	panic("unimplemented")
}

// GetRegisteredEndpointsForServiceProvider implements v1connect.EthServiceHandler.
func (e *EthServer) GetRegisteredEndpointsForServiceProvider(context.Context, *connect.Request[v1.GetRegisteredEndpointsForServiceProviderRequest]) (*connect.Response[v1.GetRegisteredEndpointsForServiceProviderResponse], error) {
	panic("unimplemented")
}

// GetServiceProvider implements v1connect.EthServiceHandler.
func (e *EthServer) GetServiceProvider(context.Context, *connect.Request[v1.GetServiceProviderRequest]) (*connect.Response[v1.GetServiceProviderResponse], error) {
	panic("unimplemented")
}

// GetServiceProviders implements v1connect.EthServiceHandler.
func (e *EthServer) GetServiceProviders(context.Context, *connect.Request[v1.GetServiceProvidersRequest]) (*connect.Response[v1.GetServiceProvidersResponse], error) {
	panic("unimplemented")
}

// GetStakingMetadataForServiceProvider implements v1connect.EthServiceHandler.
func (e *EthServer) GetStakingMetadataForServiceProvider(context.Context, *connect.Request[v1.GetStakingMetadataForServiceProviderRequest]) (*connect.Response[v1.GetStakingMetadataForServiceProviderResponse], error) {
	panic("unimplemented")
}

// GetStatus implements v1connect.EthServiceHandler.
func (e *EthServer) GetStatus(context.Context, *connect.Request[v1.GetStatusRequest]) (*connect.Response[v1.GetStatusResponse], error) {
	panic("unimplemented")
}

// IsDuplicateDelegateWallet implements v1connect.EthServiceHandler.
func (e *EthServer) IsDuplicateDelegateWallet(context.Context, *connect.Request[v1.IsDuplicateDelegateWalletRequest]) (*connect.Response[v1.IsDuplicateDelegateWalletResponse], error) {
	panic("unimplemented")
}

// Register implements v1connect.EthServiceHandler.
func (e *EthServer) Register(context.Context, *connect.Request[v1.RegisterRequest]) (*connect.Response[v1.RegisterResponse], error) {
	panic("unimplemented")
}

// Subscribe implements v1connect.EthServiceHandler.
func (e *EthServer) Subscribe(context.Context, *connect.Request[v1.SubscriptionRequest], *connect.ServerStream[v1.SubscriptionResponse]) error {
	panic("unimplemented")
}
