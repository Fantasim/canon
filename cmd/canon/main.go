package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"

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
	env := cli.Env{Stdin: os.Stdin, Stdout: stdout, Stderr: stderr, Dir: dir, NoColor: os.Getenv(noColorVar) != ""}
	env.Terminal = isTerminal(stdout)
	return cli.Main(ctx, args, env)
}

// isTerminal tells a terminal from a pipe or a file; anything but an *os.File is not one.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}
