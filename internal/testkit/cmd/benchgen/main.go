package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// options are the flags of doc.go's usage line.
type options struct {
	seed uint64
	out  string
	n    int
}

// run parses the flags and writes the project, returning the process exit code.
func run(args []string, stderr io.Writer) int {
	o := options{n: defaultN}
	fs := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Uint64Var(&o.seed, flagSeed, 0, usageSeed)
	fs.StringVar(&o.out, flagTarget, "", usageOut)
	fs.IntVar(&o.n, flagN, defaultN, usageN)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if err := validate(o, fs.NArg()); err != nil {
		_, _ = fmt.Fprintf(stderr, fmtFail, err)
		return exitUsage
	}
	if err := checkOutEmpty(o.out); err != nil {
		_, _ = fmt.Fprintf(stderr, fmtFail, err)
		return exitUsage
	}
	if err := generate(o.seed, o.out, o.n); err != nil {
		_, _ = fmt.Fprintf(stderr, fmtFail, err)
		return exitFail
	}
	return exitOK
}

// validate refuses a missing -out and a non-positive -n before anything is written.
func validate(o options, extraArgs int) error {
	if extraArgs > 0 {
		return errArgs
	}
	if o.out == "" {
		return errNoOut
	}
	if o.n <= 0 {
		return errBadN
	}
	return nil
}

// checkOutEmpty refuses an existing, non-empty -out: two seeds into one directory leave stale
// entries under a different case, duplicating table keys (E3101).
func checkOutEmpty(out string) error {
	entries, err := os.ReadDir(out)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("%w: %w", errWrite, err)
	case len(entries) > 0:
		return errOutDirty
	}
	return nil
}
