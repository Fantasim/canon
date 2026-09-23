package lane

import (
	"fmt"
	"io"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

// Lane is one engine: it produces findings for the rule ids it declares, and nothing else.
type Lane interface {
	Name() string
	Rules() []string
	Run(ctx *Context) (Result, error)
}

type Context struct {
	Repo *repo.Repo
	Go   *gosrc.Tree
	// Enabled is the set of rule ids to produce; a lane may skip work for rules not in it.
	Enabled map[string]bool
	// Toolchain is the absolute path of tools/audit/toolchain (the pinned Go tools).
	Toolchain string
	Log       io.Writer
}

func (c *Context) On(rule string) bool { return c.Enabled[rule] }

// Logf writes a progress or diagnostic line to stderr, never to the report.
func (c *Context) Logf(format string, a ...any) {
	if c.Log != nil {
		_, _ = fmt.Fprintf(c.Log, LogPrefix+format+"\n", a...)
	}
}

// Skip records a rule or a whole lane that could not run, and why (a missing tool...).
type Skip struct {
	What   string
	Reason string
}

type Result struct {
	Findings []finding.Finding
	Skipped  []Skip
}
