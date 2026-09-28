package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

type options struct {
	schema, out string
}

// run generates vm.gen.go and returns the exit code: 2 for a usage error, 1 when the schema
// is refused (nothing is written then).
func run(args []string, stderr io.Writer) int {
	opts, err := parseFlags(args, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, fmtFail, err)
		return exitUsage
	}
	if err := write(opts); err != nil {
		_, _ = fmt.Fprintf(stderr, fmtFail, err)
		return exitFail
	}
	return exitOK
}

func parseFlags(args []string, stderr io.Writer) (options, error) {
	var o options
	fs := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.schema, flagSchema, defaultSchema, usageSchema)
	fs.StringVar(&o.out, flagOutDir, defaultOutDir, usageOutDir)
	if err := fs.Parse(args); err != nil {
		return o, fmt.Errorf("flags: %w", err)
	}
	if fs.NArg() > 0 {
		return o, fmt.Errorf("%w: %v", errArgs, fs.Args())
	}
	return o, nil
}

func write(o options) error {
	data, err := os.ReadFile(o.schema)
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	src, err := generate(data, names)
	if err != nil {
		return fmt.Errorf("%s: %w", o.schema, err)
	}
	dir, err := os.OpenRoot(o.out)
	if err != nil {
		return fmt.Errorf("output directory: %w", err)
	}
	defer func() { _ = dir.Close() }()
	if err := dir.WriteFile(outFile, src, filePerm); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}
