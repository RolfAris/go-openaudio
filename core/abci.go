package core

import (
	"context"

	"github.com/cometbft/cometbft/abci/types"
)

var _ types.Application = (*Core)(nil)

// ApplySnapshotChunk implements types.Application.
func (c *Core) ApplySnapshotChunk(ctx context.Context, req *types.ApplySnapshotChunkRequest) (*types.ApplySnapshotChunkResponse, error) {
	panic("unimplemented")
}

// CheckTx implements types.Application.
func (c *Core) CheckTx(ctx context.Context, req *types.CheckTxRequest) (*types.CheckTxResponse, error) {
	panic("unimplemented")
}

// Commit implements types.Application.
func (c *Core) Commit(ctx context.Context, req *types.CommitRequest) (*types.CommitResponse, error) {
	panic("unimplemented")
}

// ExtendVote implements types.Application.
func (c *Core) ExtendVote(ctx context.Context, req *types.ExtendVoteRequest) (*types.ExtendVoteResponse, error) {
	panic("unimplemented")
}

// FinalizeBlock implements types.Application.
func (c *Core) FinalizeBlock(ctx context.Context, req *types.FinalizeBlockRequest) (*types.FinalizeBlockResponse, error) {
	panic("unimplemented")
}

// Info implements types.Application.
func (c *Core) Info(ctx context.Context, req *types.InfoRequest) (*types.InfoResponse, error) {
	panic("unimplemented")
}

// InitChain implements types.Application.
func (c *Core) InitChain(ctx context.Context, req *types.InitChainRequest) (*types.InitChainResponse, error) {
	panic("unimplemented")
}

// ListSnapshots implements types.Application.
func (c *Core) ListSnapshots(ctx context.Context, req *types.ListSnapshotsRequest) (*types.ListSnapshotsResponse, error) {
	panic("unimplemented")
}

// LoadSnapshotChunk implements types.Application.
func (c *Core) LoadSnapshotChunk(ctx context.Context, req *types.LoadSnapshotChunkRequest) (*types.LoadSnapshotChunkResponse, error) {
	panic("unimplemented")
}

// OfferSnapshot implements types.Application.
func (c *Core) OfferSnapshot(ctx context.Context, req *types.OfferSnapshotRequest) (*types.OfferSnapshotResponse, error) {
	panic("unimplemented")
}

// PrepareProposal implements types.Application.
func (c *Core) PrepareProposal(ctx context.Context, req *types.PrepareProposalRequest) (*types.PrepareProposalResponse, error) {
	panic("unimplemented")
}

// ProcessProposal implements types.Application.
func (c *Core) ProcessProposal(ctx context.Context, req *types.ProcessProposalRequest) (*types.ProcessProposalResponse, error) {
	panic("unimplemented")
}

// Query implements types.Application.
func (c *Core) Query(ctx context.Context, req *types.QueryRequest) (*types.QueryResponse, error) {
	panic("unimplemented")
}

// VerifyVoteExtension implements types.Application.
func (c *Core) VerifyVoteExtension(ctx context.Context, req *types.VerifyVoteExtensionRequest) (*types.VerifyVoteExtensionResponse, error) {
	panic("unimplemented")
}
