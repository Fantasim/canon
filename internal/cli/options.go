package cli

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	canon "github.com/fantasim/canonlang/api"
)

// options are the flags of CLI.md §2.3 and of the command.
type options struct {
	project     string
	roots       map[string]string
	format      string
	quiet       bool
	maxWarnings int
	name        string
	targets     []canon.Target
	checkFlag   bool
	diff        bool
	jsonSources bool
	adopt       []string
	layers      []string
	run         string
	verbose     bool
	depth       int
	watch       bool
}

func newOptions() *options {
	return &options{roots: map[string]string{}, format: formatText, maxWarnings: unlimited, depth: unlimited}
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
	fs.Func(flagLayer, usageLayer, o.addLayer)
	if own != nil {
		own(fs, o)
	}
	return fs
}

func initFlags(fs *flag.FlagSet, o *options) {
	fs.StringVar(&o.name, flagName, o.name, usageName)
}

// checkFlags are canon check's own flags (CLI.md §3.3), on top of the global ones.
func checkFlags(fs *flag.FlagSet, o *options) {
	fs.BoolVar(&o.watch, flagWatch, o.watch, usageWatch)
}

// buildFlags are canon build's own flags (CLI.md §3.4), on top of the global ones.
func buildFlags(fs *flag.FlagSet, o *options) {
	fs.BoolVar(&o.watch, flagWatch, o.watch, usageWatch)
	fs.Func(flagTarget, usageTarget, o.addTarget)
	fs.BoolVar(&o.checkFlag, flagCheck, o.checkFlag, usageCheck)
	fs.Func(flagAdopt, usageAdopt, o.addAdopt)
}

// testFlags are canon test's own flags (CLI.md §3.5), on top of the global ones.
func testFlags(fs *flag.FlagSet, o *options) {
	fs.StringVar(&o.run, flagRun, o.run, usageRun)
	fs.BoolVar(&o.verbose, flagVerbose, o.verbose, usageVerbose)
}

// explainFlags are canon explain's own flags (CLI.md §3.7), on top of the global ones.
func explainFlags(fs *flag.FlagSet, o *options) {
	fs.Func(flagDepth, usageDepth, o.setDepth)
}

// fmtFlags are canon fmt's own flags (CLI.md §3.6), on top of the global ones.
func fmtFlags(fs *flag.FlagSet, o *options) {
	fs.BoolVar(&o.checkFlag, flagCheck, o.checkFlag, usageFmtCheck)
	fs.BoolVar(&o.diff, flagDiff, o.diff, usageDiff)
	fs.BoolVar(&o.jsonSources, flagJSONSources, o.jsonSources, usageJSONSources)
}

// setDepth records --depth, a count of levels of parts, 0 or more.
func (o *options) setDepth(v string) (err error) {
	o.depth, err = count(v)
	return err
}

// count is a flag's count, 0 or more.
func count(v string) (int, error) {
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, errBadMax
	}
	return n, nil
}

// addLayer records one --layer, in the order given (CLI.md §2.3).
func (o *options) addLayer(v string) error {
	o.layers = append(o.layers, v)
	return nil
}

// addTarget records one --target, refusing a word CLI.md §3.4 does not list.
func (o *options) addTarget(v string) error {
	t := canon.Target(v)
	if !slices.Contains(buildTargets[:], t) {
		return fmt.Errorf(fmtQuoted, v, errBadTarget)
	}
	o.targets = append(o.targets, t)
	return nil
}

// addAdopt records one --adopt path, taken as given (CLI.md §3.4).
func (o *options) addAdopt(v string) error {
	o.adopt = append(o.adopt, v)
	return nil
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

func (o *options) setMaxWarnings(v string) (err error) {
	o.maxWarnings, err = count(v)
	return err
}

// check refuses flag values the flag package cannot judge alone.
func (o *options) check() error {
	if o.format != formatText && o.format != formatJSON {
		return fmt.Errorf(fmtQuoted, o.format, errBadFormat)
	}
	return nil
}
