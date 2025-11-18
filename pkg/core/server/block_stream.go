package server

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"go.uber.org/zap"
)

const (
	MaxBlockStreamConnections = 100
)

func (c *CoreService) StreamBlocks(ctx context.Context, req *connect.Request[v1.StreamBlocksRequest], stream *connect.ServerStream[v1.StreamBlocksResponse]) error {
	if c.blockStreamConnections.Load() >= MaxBlockStreamConnections {
		return connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("block stream connections limit reached"))
	}

	c.blockStreamConnections.Add(1)
	defer c.blockStreamConnections.Add(^uint64(0))

	blockChan := c.core.blockPubsub.Subscribe(BlockPubsubTopic, 100)
	defer c.core.blockPubsub.Unsubscribe(BlockPubsubTopic, blockChan)

	for {
		select {
		case <-ctx.Done():
			c.core.logger.Info("block stream context done")
			return ctx.Err()
		case block := <-blockChan:
			c.core.logger.Info("sending block", zap.Int64("height", block.Height))
			if err := stream.Send(&v1.StreamBlocksResponse{Block: block}); err != nil {
				c.core.logger.Error("error sending block", zap.Error(err))
			}
			c.core.logger.Info("block sent", zap.Int64("height", block.Height))
		}
	}
}
