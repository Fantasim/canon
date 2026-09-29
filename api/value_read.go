package canon

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/verify"
)

// snapshot is what a Value reads: a frozen analysis of every package, its edit snapshot, and the
// edit layer Editable is judged with. It never evaluates again (rule R4).
type snapshot struct {
	a         *build.Analysis
	s         *edit.Snapshot
	editLayer string
	layers    []string // Options.Layers, whose amendments may explain a root the checker broke
	types     *typeEncoder
	ctx       context.Context // the Value call's: a cause still to compute runs under it (API.md S11)
}

// Value returns the final value at path (rules R4-R6, API.md §6).
func (p *Project) Value(ctx context.Context, path string) (v *Value, err error) {
	defer recoverInternal(&err)
	ws, err := p.read(ctx)
	if err != nil {
		return nil, err
	}
	parsed, err := edit.Parse(path)
	if err != nil {
		return nil, syntaxError(path, err, 0)
	}
	// Every package, so P6 and Origin never depend on a selection (log-2026-09-28, edit U4b).
	a, err := analyze(ctx, ws, nil)
	if err != nil {
		return nil, err
	}
	if _, err := p.revision(ctx, ws); err != nil {
		return nil, err
	}
	s := &snapshot{a: a, s: edit.NewSnapshot(a), editLayer: p.editLayer, layers: p.layers, types: newTypeEncoder(ctx, a), ctx: ctx}
	v, err = s.resolve(path, parsed)
	s.ctx = context.WithoutCancel(ctx) // a later Child never inherits this call's cancellation
	return v, err
}

// resolve reads parsed in the snapshot; an input field has nothing at its path (EVALUATION.md §11.2).
func (s *snapshot) resolve(path string, parsed edit.Path) (*Value, error) {
	r, err := edit.Resolve(s.s, parsed)
	if err != nil {
		return nil, s.resolveError(path, parsed, err)
	}
	if r.Target == nil {
		return nil, &PathError{Op: -1, Path: path, Err: ErrInputField, Detail: inputEnv(r)}
	}
	return s.value(r)
}

// value is r's target with its type, its view-model encoding, text, origin and editability (API.md §5.2, §7).
func (s *snapshot) value(r edit.Resolved) (*Value, error) {
	t, err := s.s.Type(r)
	if err != nil {
		return nil, internalError(err)
	}
	e, err := s.s.Editable(r, edit.OpSet, s.editLayer)
	if err != nil {
		return nil, internalError(err)
	}
	v := &Value{
		Path: r.Canonical, Kind: kindOf(r.Target), Text: r.Target.CanonText(),
		Origin: s.origin(r), Editable: editabilityOf(e), snap: s, res: r,
	}
	if t == nil {
		return v, nil
	}
	v.Type.Expr = t.String()
	if v.Type.VM, err = s.types.expr(s.siteOf(r), t); err != nil {
		return nil, internalError(err)
	}
	return v, nil
}

// syntaxError is ErrBadPath with the reason and byte, shifted for a prefix it was parsed after (API.md §6.1).
func syntaxError(path string, err error, shift int) error {
	var se *edit.SyntaxError
	if !errors.As(err, &se) {
		return internalError(err)
	}
	return &PathError{Op: -1, Path: path, Err: ErrBadPath, Detail: fmt.Sprintf(fmtAtByte, se.Reason, se.Offset+shift)}
}

// resolveError is edit's refusal as the API's error, any other one internal (API.md §15, R5, R6).
func (s *snapshot) resolveError(path string, parsed edit.Path, err error) error {
	var pe *edit.PathError
	if !errors.As(err, &pe) {
		return internalError(err)
	}
	i := slices.IndexFunc(pathSentinels[:], func(m sentinelMap) bool { return errors.Is(err, m.from) })
	if i < 0 {
		return internalError(err)
	}
	out := &PathError{Op: -1, Path: path, Err: pathSentinels[i].to, Detail: segmentDetail(parsed, pe.Seg)}
	if errors.Is(out.Err, ErrAmbiguousPath) {
		out.Candidates, out.Detail = pe.Candidates, strings.Join(pe.Candidates, textListSep)
	}
	if errors.Is(out.Err, ErrNoValue) {
		var err error
		if out.Findings, err = s.cause(pe.Root); err != nil {
			return err
		}
	}
	return out
}

// segmentDetail names the segment a path failed at, or its root (rule P5).
func segmentDetail(p edit.Path, seg int) string {
	if seg < 0 || seg >= len(p.Segs) {
		return fmt.Sprintf(fmtRoot, edit.Path{Package: p.Package, Root: p.Root})
	}
	return fmt.Sprintf(fmtSegment, edit.Path{Segs: p.Segs[seg : seg+1]})
}

// inputEnv is the environment variable of the input field r ends at (EVALUATION.md §11.2).
func inputEnv(r edit.Resolved) string {
	if len(r.Steps) == 0 {
		return ""
	}
	last := r.Steps[len(r.Steps)-1]
	for _, f := range verify.Fields(last.Container) {
		if f.Name == last.Seg.Name && f.Input != nil {
			return f.Input.Env
		}
	}
	return ""
}

// internalError is a compiler bug met at the API boundary, with the Go stack (rule X2).
func internalError(err error) error {
	return &InternalError{Msg: err.Error(), Stack: string(debug.Stack())}
}

// withSeg is p with seg appended, p's segments untouched.
func withSeg(p edit.Path, seg edit.Seg) edit.Path {
	return edit.Path{Package: p.Package, Root: p.Root, Segs: append(slices.Clip(p.Segs), seg)}
}
