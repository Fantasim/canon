package eval

import (
	"context"
	"regexp"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// Builder verifies an expect subject and runs its instance checks (EVALUATION.md §10.2).
type Builder interface {
	Build(ctx context.Context, v value.Value) []diag.Finding
}

// TestRun is the outcome of one test (EVALUATION.md §10.4).
type TestRun struct {
	Failed   bool     // an expect failed, or the test stopped
	Stopped  bool     // a hard error outside a subject, or a poisoned read, stopped it
	Broken   bool     // a static error: the test did not run (TYPES.md §1)
	Poisoned string   // the poisoned top-level value whose read stopped it, if any (§7.2)
	Expects  []Expect // the expects run, in order
}

// Expect is one expect run: whether it passed, the findings its subject captured, and the
// poisoned top-level value it read, if any.
type Expect struct {
	Stmt     *syntax.ExpectStmt
	Passed   bool
	Captured []diag.Finding
	Poisoned string
}

// testState is the test a run executes.
type testState struct {
	builder Builder
	out     *TestRun
}

var codeShape = regexp.MustCompile(codePattern)

// Test runs a test block (EVALUATION.md §10).
func (e *Evaluator) Test(ctx context.Context, t *syntax.TestDecl, b Builder) TestRun {
	file := e.index.file[t]
	if file == nil || e.info == nil || e.broken(e.index.decls[t]) {
		return TestRun{Failed: true, Broken: true}
	}
	if e.exhausted {
		return TestRun{Failed: true, Stopped: true}
	}
	var out TestRun
	r := e.newRun(ctx, charge{pkg: e.index.pkg[file], name: testName(t)}, file)
	r.test = &testState{builder: b, out: &out}
	if r.block(t.Body) == flowAbort || r.failed {
		out.Failed, out.Stopped, out.Poisoned = true, true, r.poisonAt
	}
	return out
}

func testName(t *syntax.TestDecl) string {
	switch n := t.Name.(type) {
	case *syntax.StringLit:
		return testNameParts(n)
	case *syntax.RawStringLit:
		return n.Value
	}
	return ""
}

// execExpect builds its subject with its findings captured, then judges it (§10.3).
func execExpect(r *run, s syntax.Stmt) flow {
	x := s.(*syntax.ExpectStmt)
	if r.test == nil {
		r.bug(s)
		return flowAbort
	}
	capture := diag.NewBag(r.ev.files(), r.fr.pkg)
	saved := r.sink
	r.sink, r.poisonAt = capture, ""
	v := r.eval(x.X)
	aborted := r.failed
	r.failed, r.sink = false, saved
	ex := Expect{Stmt: x, Poisoned: r.poisonAt}
	if !aborted && x.Outcome != nil && r.test.builder != nil {
		ex.Captured = r.test.builder.Build(r.ctx, v)
	}
	ex.Captured = unique(append(capture.Findings(), ex.Captured...))
	r.poisonAt = ""
	if r.ev.exhausted {
		r.failed = true
		return flowAbort
	}
	ex.Passed = ex.Poisoned == "" && judge(x, v, aborted, ex.Captured)
	r.test.out.Expects = append(r.test.out.Expects, ex)
	if !ex.Passed {
		r.test.out.Failed = true
	}
	return flowNext
}

// unique drops the findings a conversion and verification both reported (EVALUATION.md §14).
func unique(fs []diag.Finding) []diag.Finding {
	type key struct {
		code    diag.Code
		span    source.Span
		message string
	}
	seen := map[key]bool{}
	var out []diag.Finding
	for _, f := range fs {
		k := key{code: f.Code, span: f.Span, message: f.Message}
		if !seen[k] {
			seen[k] = true
			out = append(out, f)
		}
	}
	return out
}

// judge is whether an expect passes (EVALUATION.md §10.3).
func judge(x *syntax.ExpectStmt, v value.Value, aborted bool, captured []diag.Finding) bool {
	if x.Outcome == nil {
		b, ok := v.(*value.Bool)
		return !aborted && ok && b.V && !any(captured, diag.Error, nil)
	}
	switch x.Outcome.Name {
	case outcomePasses:
		return !any(captured, diag.Error, nil)
	case outcomeFails:
		return any(captured, diag.Error, matcher(x.Message))
	case outcomeWarns:
		return any(captured, diag.Warning, matcher(x.Message))
	}
	return false
}

// matcher is what `fails …` and `warns …` look for: a message substring, a code, or a check.
func matcher(m syntax.NameLit) func(f diag.Finding) bool {
	switch n := m.(type) {
	case *syntax.StringLit:
		text := testNameParts(n)
		return func(f diag.Finding) bool { return strings.Contains(f.Message, text) }
	case *syntax.RawStringLit:
		return func(f diag.Finding) bool { return strings.Contains(f.Message, n.Value) }
	case *syntax.Ident:
		if codeShape.MatchString(n.Name) {
			return func(f diag.Finding) bool { return string(f.Code) == n.Name }
		}
		return func(f diag.Finding) bool { return f.Check == n.Name }
	}
	return nil
}

func testNameParts(n *syntax.StringLit) string {
	var b strings.Builder
	for _, p := range n.Parts {
		b.WriteString(p.Text)
	}
	return b.String()
}

// any reports a captured finding of severity sev that match accepts (every one when nil).
func any(fs []diag.Finding, sev diag.Severity, match func(diag.Finding) bool) bool {
	for _, f := range fs {
		if f.Severity == sev && (match == nil || match(f)) {
			return true
		}
	}
	return false
}

// files resolves the spans of the program's files, for a capture's bag.
func (e *Evaluator) files() diag.Files {
	fs := fileSet{}
	for f := range e.index.pkg { //canon:unordered a lookup table
		fs[f.Src.ID] = f.Src
	}
	return fs
}

// fileSet is the diag.Files of the parsed files, by id.
type fileSet map[source.FileID]*source.File

func (s fileSet) Path(id source.FileID) string {
	if f := s[id]; f != nil {
		return f.Path
	}
	return ""
}

func (s fileSet) Position(id source.FileID, p source.Pos) (int, int) {
	if f := s[id]; f != nil {
		return f.Position(p)
	}
	return 0, 0
}

func (s fileSet) Content(id source.FileID) []byte {
	if f := s[id]; f != nil {
		return f.Content
	}
	return nil
}
