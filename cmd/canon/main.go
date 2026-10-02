package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/fantasim/canonlang/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run is the command line in the current directory; an interrupt cancels ctx (CLI.md §2.5).
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	dir, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitInternal
	}
	return cli.Main(ctx, args, cli.Env{Stdin: os.Stdin, Stdout: stdout, Stderr: stderr, Dir: dir})
}
