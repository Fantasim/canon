package eval

import (
	"context"
	"math"
	"regexp"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// Builder verifies an expect subject and runs its checks into the capture (DECISIONS 197).
type Builder interface {
	Build(ctx context.Context, v value.Value, capture *diag.Bag)
}

// TestRun is the outcome of one test (EVALUATION.md §10.4).
type TestRun struct {
	Failed     bool           // an expect failed, or the test stopped
	Stopped    bool           // a hard error outside a subject, or a poisoned read, stopped it
	Broken     bool           // a static error: the test did not run (TYPES.md §1)
	Poisoned   string         // the poisoned top-level value whose read stopped it, if any (§7.2)
	PoisonRoot Root           // that value
	PoisonSpan source.Span    // where the test read it
	Errors     []diag.Finding // the hard error that stopped it outside a subject, E4401 included, each in its frame's package (CLI.md §3.5)
	Expects    []Expect       // the expects run, in order
}

// Expect is one expect run: whether it passed, the findings its subject captured, and the
// poisoned top-level value it read, if any.
type Expect struct {
	Stmt        *syntax.ExpectStmt
	Passed      bool
	Captured    []diag.Finding
	Poisoned    string
	PoisonRoot  Root
	Compared    bool             // a failed comparison whose two operands were evaluated
	Left, Right string           // its operands as text, up to operandText bytes, possibly empty (EVALUATION.md §10.3)
	Op          syntax.TokenKind // its operator
	Value       string           // a failed `expect c` that is no comparison: c's value as text, when it has one
}

// comparison is the operands of the comparison an `expect c` evaluates.
type comparison struct {
	at          syntax.Expr
	op          syntax.TokenKind
	left, right value.Value
}

// testState is the test a run executes.
type testState struct {
	builder Builder
	out     *TestRun
}

var codeShape = regexp.MustCompile(codePattern)

// Test runs a test block (EVALUATION.md §10).
func (e *Evaluator) Test(ctx context.Context, t *syntax.TestDecl, b Builder) TestRun {
	file := e.fileOf(t)
	if file == nil || e.info == nil || e.broken(e.index.decls[t]) {
		return TestRun{Failed: true, Broken: true}
	}
	pkg := e.index.pkg[file]
	if e.halted(pkg) {
		return TestRun{Failed: true, Stopped: true}
	}
	var out TestRun
	e.beginTestLog(pkg)
	defer func() { e.testStops, e.testPkg = nil, "" }()
	r := e.newRun(ctx, charge{pkg: pkg, name: TestName(t)}, file)
	r.test = &testState{builder: b, out: &out}
	if r.block(t.Body) == flowAbort || r.failed {
		out.Failed, out.Stopped = true, true
		out.Poisoned, out.PoisonRoot, out.PoisonSpan = r.poisonAt, r.poisonRoot, r.poisonSpan
	}
	out.Errors = e.stopFindings()
	return out
}

// TestName is the name a test block declares (EVALUATION.md §10.1).
func TestName(t *syntax.TestDecl) string {
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
	b := r.subject(x)
	if r.ev.halted(r.charge.pkg) {
		r.failed = true
		return flowAbort
	}
	ex := b.ex
	ex.Passed = ex.Poisoned == "" && judge(x, b.v, b.aborted, ex.Captured)
	if !ex.Passed {
		r.test.out.Failed = true
		describe(&ex, b)
	}
	r.test.out.Expects = append(r.test.out.Expects, ex)
	return flowNext
}

// describe adds a failed expect's comparison operands, or its Boolean's value (CLI.md §3.5).
func describe(ex *Expect, b built) {
	switch {
	case b.cmp != nil && b.cmp.left != nil && b.cmp.right != nil:
		ex.Left, ex.Right = value.TextUpTo(b.cmp.left, operandText), value.TextUpTo(b.cmp.right, operandText)
		ex.Op, ex.Compared = b.cmp.op, true
	case b.cmp == nil && ex.Stmt.Outcome == nil && !b.aborted && b.v != nil:
		ex.Value = value.TextUpTo(b.v, operandText)
	}
}

// built is an expect's subject once built: the expect so far, its value, whether evaluating
// it aborted, and the comparison it evaluated, if any.
type built struct {
	ex      Expect
	v       value.Value
	aborted bool
	cmp     *comparison
}

// subject builds an expect's subject into one capture, unlimited and deduplicated (DECISIONS 197).
func (r *run) subject(x *syntax.ExpectStmt) built {
	capture := diag.NewBag(r.ev.files(), r.fr.pkg)
	capture.Truncate(math.MaxInt)
	saved := r.sink
	b := built{cmp: comparisonOf(x)}
	r.sink, r.poisonAt, r.cmp = capture, "", b.cmp
	b.v = r.eval(x.X)
	b.aborted = r.failed
	r.failed, r.sink, r.cmp = false, saved, nil
	b.ex = Expect{Stmt: x, Poisoned: r.poisonAt, PoisonRoot: r.poisonRoot}
	r.poisonAt, r.poisonRoot, r.poisonSpan = "", Root{}, source.Span{}
	if !b.aborted && x.Outcome != nil && r.test.builder != nil {
		r.test.builder.Build(r.ctx, b.v, capture)
	}
	b.ex.Captured = capture.Findings()
	return b
}

// comparisonOf is the comparison `expect c` evaluates, nil when c is not one.
func comparisonOf(x *syntax.ExpectStmt) *comparison {
	if x.Outcome != nil {
		return nil
	}
	if c, ok := syntax.Unparen(x.X).(*syntax.BinaryExpr); ok && comparisons[c.Op] {
		return &comparison{at: c, op: c.Op}
	}
	return nil
}

// judge is whether an expect passes (EVALUATION.md §10.3).
func judge(x *syntax.ExpectStmt, v value.Value, aborted bool, captured []diag.Finding) bool {
	if x.Outcome == nil {
		b, ok := v.(*value.Bool)
		return !aborted && ok && b.V && !anyFinding(captured, diag.Error, nil)
	}
	switch x.Outcome.Name {
	case outcomePasses:
		return !anyFinding(captured, diag.Error, nil)
	case outcomeFails:
		return anyFinding(captured, diag.Error, matcher(x.Message))
	case outcomeWarns:
		return anyFinding(captured, diag.Warning, matcher(x.Message))
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

// anyFinding reports a captured finding of severity sev that match accepts (every one when nil).
func anyFinding(fs []diag.Finding, sev diag.Severity, match func(diag.Finding) bool) bool {
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
