package main

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/OpenAudio/go-openaudio/pkg/sdk"
)

func main() {
	openaudio := sdk.NewOpenAudioSDK("node1.oap.devnet")

	stream, err := openaudio.Core.StreamBlocks(context.Background(), connect.NewRequest(&v1.StreamBlocksRequest{}))
	if err != nil {
		log.Fatal(err)
	}

	for {
		fmt.Println("receiving block")
		ok := stream.Receive()
		if !ok {
			log.Fatal("stream closed")
		}

		fmt.Println("received block")
		msg := stream.Msg()
		if msg == nil {
			log.Fatal("stream message is nil")
		}

		fmt.Println(msg.Block.Height)
	}
}
