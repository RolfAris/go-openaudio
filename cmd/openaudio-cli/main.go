package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/OpenAudio/go-openaudio/pkg/commands"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := commands.Execute(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "\nerror: %v\n\n%s\n", err, debug.Stack())
		os.Exit(1)
	}
}
