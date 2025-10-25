package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/OpenAudio/go-openaudio/pkg/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "\nerror: %v\n\n%s\n", err, debug.Stack())
		os.Exit(1)
	}
}
