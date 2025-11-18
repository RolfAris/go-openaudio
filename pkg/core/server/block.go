package server

import (
	"context"
	"fmt"

	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/OpenAudio/go-openaudio/pkg/api/core/v1beta1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// getBlock returns a block and its transactions from the database and converts them to v1.Block and v1.Transaction objectss
func (s *Server) getBlock(ctx context.Context, height int64) (*v1.Block, error) {
	blockRows, err := s.db.GetBlockWithTransactions(ctx, height)
	if err != nil {
		return nil, err
	}
	if len(blockRows) == 0 {
		return nil, fmt.Errorf("block not found")
	}

	blockRow := blockRows[0]

	block := &v1.Block{
		Hash:      blockRow.BlockHash,
		ChainId:   blockRow.ChainID,
		Proposer:  blockRow.Proposer,
		Height:    blockRow.Height,
		Timestamp: timestamppb.New(blockRow.BlockCreatedAt.Time),
	}

	if len(blockRows) > 1 {
		txs := []*v1.Transaction{}

		for _, txRow := range blockRows[1:] {
			tx := &v1.Transaction{
				Hash:      txRow.TxHash.String,
				BlockHash: blockRow.BlockHash,
				ChainId:   blockRow.ChainID,
				Height:    blockRow.Height,
				Timestamp: timestamppb.New(txRow.TxCreatedAt.Time),
			}

			var v1tx v1.SignedTransaction
			if err := proto.Unmarshal(txRow.Transaction, &v1tx); err == nil {
				tx.Transaction = &v1tx
			} else {
				var v2tx v1beta1.Transaction
				if err := proto.Unmarshal(txRow.Transaction, &v2tx); err != nil {
					return nil, fmt.Errorf("error unmarshaling transaction as v1 or v2: %v", err)
				}
				tx.Transactionv2 = &v2tx
			}

			txs = append(txs, tx)
		}

		block.Transactions = txs
	}

	return block, nil
}
