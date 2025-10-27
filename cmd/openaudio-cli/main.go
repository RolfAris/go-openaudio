package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/OpenAudio/go-openaudio/pkg/commands"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := commands.Execute(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			// Graceful shutdown (Ctrl-C or SIGTERM)
			fmt.Fprintln(os.Stderr, "shutdown requested, exiting cleanly")
			os.Exit(0)
		}

		// Real error
		fmt.Fprintf(os.Stderr, "\nerror: %v\n\n%s\n", err, debug.Stack())
		os.Exit(1)
	}
}
