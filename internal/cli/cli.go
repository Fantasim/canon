package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
)

// Env is the process as a command sees it: its output streams and current directory.
type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	Dir    string
}

// command is one command of CLI.md §3: its own flags, then its run.
type command struct {
	flags func(fs *flag.FlagSet, o *options)
	run   func(inv *invocation) int
}

// commands are the commands this compiler implements, by name.
func commands() map[string]command {
	return map[string]command{
		cmdCheck:   {run: runCheck},
		cmdInit:    {flags: initFlags, run: runInit},
		cmdNew:     {run: runNew},
		cmdVersion: {run: runVersion},
	}
}

// invocation is one command being run.
type invocation struct {
	ctx  context.Context
	env  Env
	opt  *options
	args []string
}

// Main runs a command line, without the program name, and returns its exit code (CLI.md §2.5).
func Main(ctx context.Context, args []string, env Env) int {
	o := newOptions()
	global := newFlagSet(o, nil)
	if err := global.Parse(args); err != nil {
		return usageError(env, global, err)
	}
	if global.NArg() == 0 {
		return usageError(env, global, errNoCommand)
	}
	name, rest := global.Arg(0), global.Args()[1:]
	cmd, ok := commands()[name]
	if !ok {
		return usageError(env, global, fmt.Errorf(fmtQuoted, name, errUnknownCommand))
	}
	fs := newFlagSet(o, cmd.flags)
	positional, err := parseInterspersed(fs, rest)
	if err != nil {
		return usageError(env, fs, err)
	}
	if err := o.check(); err != nil {
		return usageError(env, fs, err)
	}
	inv := &invocation{ctx: ctx, env: env, opt: o, args: positional}
	code := cmd.run(inv)
	if ctx.Err() != nil {
		return inv.fail(ctx.Err())
	}
	return code
}

// parseInterspersed parses flags anywhere among the arguments; after "--" all are arguments.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, fmt.Errorf(fmtWrap, err)
		}
		rest := fs.Args()
		consumed := len(args) - len(rest)
		if len(rest) == 0 || consumed > 0 && args[consumed-1] == endOfFlags {
			return append(positional, rest...), nil
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
}

// usageError writes err and the usage to stderr (exit 2).
func usageError(env Env, fs *flag.FlagSet, err error) int {
	writeLine(env.Stderr, msgPrefix+err.Error())
	return printUsage(env.Stderr, fs, exitUsage)
}

func printUsage(w io.Writer, fs *flag.FlagSet, code int) int {
	_, _ = io.WriteString(w, usageText)
	fs.SetOutput(w)
	fs.PrintDefaults()
	fs.SetOutput(io.Discard)
	return code
}

func writeLine(w io.Writer, s string) {
	_, _ = io.WriteString(w, s+lineBreak)
}

// abs is dir under the current directory unless absolute; "" is the current directory.
func (inv *invocation) abs(dir string) string {
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	return filepath.Join(inv.env.Dir, dir)
}
