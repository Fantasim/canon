package build

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// checkRuns keeps the named one-line checks that failed in stages C and D, with the instance
// each ran on, so a view model can render their translated messages (VIEWMODEL.md J15).
type checkRuns struct {
	prog  *check.Program
	runs  map[*syntax.CheckDecl][]failedRun
	decls map[source.Span]*syntax.CheckDecl // each check declaration by its span, built on first find
}

// failedRun is one failed run of a check: its instance (nil at package level), the path its
// finding carries, as rules wrote it ("" at package level), and message.
type failedRun struct {
	self    value.Value
	path    string
	message string
}

// note keeps a run of c on self at path whose message may be translated (I18N.md K8: a named one-line check).
func (l *checkRuns) note(c *syntax.CheckDecl, self value.Value, path, message string) {
	if l == nil || c.Name == nil || c.Body != nil {
		return
	}
	if l.runs == nil {
		l.runs = map[*syntax.CheckDecl][]failedRun{}
	}
	l.runs[c] = append(l.runs[c], failedRun{self: self, path: path, message: message})
}

// find is the check f relates and its run with f's message on the instance f is about (API.md F1).
func (l *checkRuns) find(f diag.Finding) (*syntax.CheckDecl, value.Value, bool) {
	if l == nil || len(l.runs) == 0 || f.Check == "" {
		return nil, nil, false
	}
	for _, rel := range f.Related {
		c := l.declAt(rel.Span)
		if c == nil || c.Name == nil || c.Name.Name != f.Check {
			continue
		}
		for _, run := range l.runs[c] {
			if run.message == f.Message && run.path == f.Path && (run.self == nil || siteOf(c, run.self) == f.Span) {
				return c, run.self, true
			}
		}
	}
	return nil, nil, false
}

// declAt is the check declared at span, indexing every check of the program on first use.
func (l *checkRuns) declAt(span source.Span) *syntax.CheckDecl {
	if l.decls == nil {
		l.decls = map[source.Span]*syntax.CheckDecl{}
		for _, cp := range l.prog.Packages {
			for _, f := range cp.Files {
				l.indexChecks(f)
			}
		}
	}
	return l.decls[span]
}

// indexChecks records every check declaration of f by its span.
func (l *checkRuns) indexChecks(f *syntax.File) {
	syntax.Inspect(f, func(n syntax.Node) bool {
		if c, ok := n.(*syntax.CheckDecl); ok {
			l.decls[f.Span(c)] = c
		}
		return true
	})
}

// siteOf locates an instance check's finding as rules does; none for another value.
func siteOf(c *syntax.CheckDecl, self value.Value) source.Span {
	if rec, ok := self.(*value.Record); ok {
		v, _ := rules.Placed(c, rec)
		return verify.SiteOf(v).Span
	}
	return source.Span{}
}
