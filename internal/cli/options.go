package cli

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

// options are the flags of CLI.md §2.3 and of the command.
type options struct {
	project     string
	roots       map[string]string
	format      string
	quiet       bool
	maxWarnings int
	name        string
}

func newOptions() *options {
	return &options{roots: map[string]string{}, format: formatText, maxWarnings: unlimited}
}

// newFlagSet is the global flags of CLI.md §2.3, then the command's own.
func newFlagSet(o *options, own func(*flag.FlagSet, *options)) *flag.FlagSet {
	fs := flag.NewFlagSet(progName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	fs.StringVar(&o.project, flagProject, o.project, usageProject)
	fs.Func(flagRoot, usageRoot, o.addRoot)
	fs.StringVar(&o.format, flagFormat, o.format, usageFormat)
	fs.BoolVar(&o.quiet, flagQuiet, o.quiet, usageQuiet)
	fs.BoolVar(&o.quiet, flagQuietShort, o.quiet, usageQuiet)
	fs.Func(flagMaxWarnings, usageMaxWarnings, o.setMaxWarnings)
	if own != nil {
		own(fs, o)
	}
	return fs
}

func initFlags(fs *flag.FlagSet, o *options) {
	fs.StringVar(&o.name, flagName, o.name, usageName)
}

// addRoot records one --root name=dir; a name given twice is refused.
func (o *options) addRoot(v string) error {
	name, dir, ok := strings.Cut(v, rootAssign)
	if !ok || name == "" || dir == "" {
		return errBadRoot
	}
	if _, twice := o.roots[name]; twice {
		return errRootTwice
	}
	o.roots[name] = filepath.ToSlash(dir)
	return nil
}

func (o *options) setMaxWarnings(v string) error {
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return errBadMax
	}
	o.maxWarnings = n
	return nil
}

// check refuses flag values the flag package cannot judge alone.
func (o *options) check() error {
	if o.format != formatText && o.format != formatJSON {
		return fmt.Errorf(fmtQuoted, o.format, errBadFormat)
	}
	return nil
}
