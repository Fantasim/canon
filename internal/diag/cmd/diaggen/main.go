package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/fantasim/canonlang/internal/diag/catalog"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

type options struct {
	errors, plan, out, runtime string
}

// run generates the registry and returns the exit code: 2 for a usage error, 1 when the
// catalogue or a runtime text is refused (nothing is written then).
func run(args []string, stderr io.Writer) int {
	opts, err := parseFlags(args, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, fmtFail, err)
		return exitUsage
	}
	if err := generate(opts); err != nil {
		_, _ = fmt.Fprintf(stderr, fmtFail, err)
		return exitFail
	}
	return exitOK
}

func parseFlags(args []string, stderr io.Writer) (options, error) {
	var o options
	fs := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.errors, flagErrors, defaultErrors, usageErrors)
	fs.StringVar(&o.plan, flagPlan, defaultPlan, usagePlan)
	fs.StringVar(&o.out, flagOut, defaultOut, usageOut)
	fs.StringVar(&o.runtime, flagRuntime, "", usageRuntime)
	if err := fs.Parse(args); err != nil {
		return o, fmt.Errorf("flags: %w", err)
	}
	if fs.NArg() > 0 {
		return o, fmt.Errorf("%w: %v", errArgs, fs.Args())
	}
	return o, nil
}

// generate parses, generates and checks everything before it writes the first file.
func generate(o options) error {
	errorsMD, err := os.ReadFile(o.errors)
	if err != nil {
		return fmt.Errorf("read catalogue: %w", err)
	}
	planMD, err := os.ReadFile(o.plan)
	if err != nil {
		return fmt.Errorf("read plan: %w", err)
	}
	cat, err := catalog.Parse(errorsMD, planMD)
	if err != nil {
		return fmt.Errorf("%s: %w", o.errors, err)
	}
	files, err := cat.Generate()
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	if o.runtime != "" {
		if err := checkRuntime(cat, o.runtime); err != nil {
			return err
		}
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(o.out, f.Name), f.Data, filePerm); err != nil {
			return fmt.Errorf("write: %w", err)
		}
	}
	return nil
}

func checkRuntime(cat *catalog.Catalog, dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("runtime texts: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("runtime texts: %s: %w", dir, errRuntimeDir)
	}
	if err := cat.CheckRuntime(os.DirFS(dir)); err != nil {
		return fmt.Errorf("runtime texts under %s: %w", dir, err)
	}
	return nil
}
