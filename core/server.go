package core

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/openaudio/v1"
	"github.com/OpenAudio/go-openaudio/pkg/api/openaudio/v1/v1connect"
)

var _ v1connect.CoreHandler = (*Core)(nil)

// GetBlock implements v1connect.CoreHandler.
func (c *Core) GetBlock(context.Context, *connect.Request[v1.GetBlockRequest]) (*connect.Response[v1.GetBlockResponse], error) {
	panic("unimplemented")
}

// GetBlocks implements v1connect.CoreHandler.
func (c *Core) GetBlocks(context.Context, *connect.Request[v1.GetBlocksRequest]) (*connect.Response[v1.GetBlocksResponse], error) {
	panic("unimplemented")
}

// GetTransaction implements v1connect.CoreHandler.
func (c *Core) GetTransaction(context.Context, *connect.Request[v1.GetTransactionRequest]) (*connect.Response[v1.GetTransactionResponse], error) {
	panic("unimplemented")
}

// SendTransaction implements v1connect.CoreHandler.
func (c *Core) SendTransaction(context.Context, *connect.Request[v1.SendTransactionRequest]) (*connect.Response[v1.SendTransactionResponse], error) {
	panic("unimplemented")
}

// StreamTransactions implements v1connect.CoreHandler.
func (c *Core) StreamTransactions(context.Context, *connect.Request[v1.StreamTransactionsRequest], *connect.ServerStream[v1.StreamTransactionsResponse]) error {
	panic("unimplemented")
}

// StreamBlocks implements v1connect.CoreHandler.
func (c *Core) StreamBlocks(context.Context, *connect.Request[v1.StreamBlocksRequest], *connect.ServerStream[v1.StreamBlocksResponse]) error {
	panic("unimplemented")
}
