package main

import (
	_ "embed"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

//go:embed fixtures.tsv
var manifest string

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

type options struct {
	real, out string
}

// run extracts the fixtures and returns the exit code: 2 for a usage error, 1 when anything
// fails (nothing is written then).
func run(args []string, stderr io.Writer) int {
	var o options
	fs := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.real, flagReal, defaultReal, usageReal)
	fs.StringVar(&o.out, flagOut, defaultOut, usageOut)
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, fmtFail, errArgs)
		return exitUsage
	}
	if err := extract(o, manifest); err != nil {
		_, _ = fmt.Fprintf(stderr, fmtFail, err)
		return exitFail
	}
	return exitOK
}

// extract reads and extracts every fixture of the manifest, checks the size budget, and only
// then writes them.
func extract(o options, manifest string) error {
	if info, err := os.Stat(o.real); err != nil || !info.IsDir() {
		return fmt.Errorf("%w: %s", errNoRealData, o.real)
	}
	rows, err := parseManifest(manifest)
	if err != nil {
		return err
	}
	outputs := make([][]byte, len(rows))
	for i, r := range rows {
		src, err := os.ReadFile(filepath.Join(o.real, filepath.FromSlash(r.path)))
		if err != nil {
			return fmt.Errorf("%w: %w", errExtract, err)
		}
		if outputs[i], err = methods[r.method](src, r.names); err != nil {
			return fmt.Errorf("%s: %w", r.path, err)
		}
	}
	if err := checkBudget(o.out, rows, outputs); err != nil {
		return err
	}
	return write(o.out, rows, outputs)
}

func write(out string, rows []row, outputs [][]byte) error {
	for i, r := range rows {
		path := filepath.Join(out, filepath.FromSlash(r.path))
		if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
			return fmt.Errorf("write: %w", err)
		}
		if err := os.WriteFile(path, outputs[i], filePerm); err != nil {
			return fmt.Errorf("write: %w", err)
		}
	}
	return nil
}
