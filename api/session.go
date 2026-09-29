package canon

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/workspace"
	"github.com/fantasim/canonlang/internal/workspace/safego"
)

// absolute is dir made absolute and '/'-separated, the form of FS names (API.md §2.2).
func absolute(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf(fmtWrap, ErrNoProject, err)
	}
	return filepath.ToSlash(abs), nil
}

// workspace is the project's snapshots, made over its build on first use (API.md §3).
func (p *Project) workspace() *workspace.Project {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ws == nil {
		p.ws = workspace.New(p.b)
	}
	return p.ws
}

// read is the snapshot a call runs against, refreshed (rule S1), or ErrClosed after Close (O6).
func (p *Project) read(ctx context.Context) (*workspace.Snapshot, error) {
	s, err := p.workspace().Read(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	return s, nil
}

// share is fn's result on s, one computation for identical concurrent calls (rule S8).
func share[T any](ctx context.Context, s *workspace.Snapshot, key string, fn func(context.Context) (T, error)) (T, error) {
	v, err := workspace.Share(ctx, s, key, fn)
	if err != nil {
		return v, apiError(err)
	}
	return v, nil
}

// analyze is the analysis of the selected packages on snapshot s, shared (rule S8).
func analyze(ctx context.Context, s *workspace.Snapshot, selectors []string) (*build.Analysis, error) {
	return share(ctx, s, workspace.Key(workspace.OpAnalyze, selectors), func(ctx context.Context) (*build.Analysis, error) {
		return s.Build().Analyze(ctx, selectors)
	})
}

// revision is s's revision once a call has read what it needed (rule S3), kept as the last
// revision read.
func (p *Project) revision(ctx context.Context, s *workspace.Snapshot) (Revision, error) {
	rev, err := s.Revision(ctx)
	if err != nil {
		return "", err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rev = Revision(rev)
	return p.rev, nil
}

// apiError is an error of build as the API reports it (rules O1-O4, R1, R3, S5, X2):
// project.canon's errors as a *ProjectError with their findings, whether Open or a later call
// met them, a stale base as a *StaleError, and a compiler bug as an *InternalError.
func apiError(err error) error {
	var oe *build.OpenError
	var ue *project.UnknownError
	switch {
	case errors.As(err, &oe):
		return &ProjectError{Err: projectSentinel(err), Findings: fromDiag(oe.Findings.Files, oe.Findings.List)}
	case errors.As(err, &ue) && errors.Is(err, project.ErrMixedDirectory):
		return fmt.Errorf(fmtMixed, ErrUnknownPackage, ue.Name, project.ErrMixedDirectory)
	case errors.As(err, &ue):
		return fmt.Errorf(fmtUnknown, ErrUnknownPackage, ue.Name)
	case errors.Is(err, build.ErrInternal):
		return internalError(err)
	case errors.Is(err, workspace.ErrClosed):
		return ErrClosed
	case errors.Is(err, workspace.ErrStale):
		return staleError(err)
	}
	return panicError(err)
}

// staleError is workspace's staleness as the API reports it, the same files (rules S4, S5).
func staleError(err error) error {
	var se *workspace.StaleError
	if errors.As(err, &se) {
		return &StaleError{Files: slices.Clone(se.Files)}
	}
	return err
}

// panicError is a panic recovered on a shared computation's goroutine as the *InternalError
// the call would have returned had it panicked itself (rule X2).
func panicError(err error) error {
	var pe *safego.PanicError
	if errors.As(err, &pe) {
		return &InternalError{Msg: fmt.Sprint(pe.Value), Stack: pe.Stack}
	}
	return err
}

func projectSentinel(err error) error {
	switch {
	case errors.Is(err, project.ErrNoProject):
		return ErrNoProject
	case errors.Is(err, project.ErrUnsupportedVersion):
		return ErrUnsupportedVersion
	case errors.Is(err, build.ErrUnknownLayer): // E1901, with its findings (log "canon test review calls")
		return ErrUnknownLayer
	}
	return ErrProject
}

// recoverInternal turns a panic into the call's *InternalError (rule X2).
func recoverInternal(err *error) {
	if r := recover(); r != nil {
		*err = &InternalError{Msg: fmt.Sprint(r), Stack: string(debug.Stack())}
	}
}

// fromDiag converts findings to the API's form, in rule F2 order.
func fromDiag(files diag.Files, list []diag.Finding) []Finding {
	out := make([]Finding, 0, len(list))
	for _, l := range diag.Locate(files, list) {
		out = append(out, fromLocated(l))
	}
	slices.SortStableFunc(out, func(a, b Finding) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col),
			cmp.Compare(a.Code, b.Code), cmp.Compare(a.Message, b.Message))
	})
	return out
}

func fromLocated(l diag.Located) Finding {
	f := Finding{
		Severity: SeverityError, Code: string(l.Code), Span: spanOf(l.Loc),
		Pointer: l.Pointer, Package: l.Package, Path: l.Path, Message: l.Message,
		Check: l.Check, Layer: l.Layer, MoreFrames: l.MoreFrames, Reads: l.Reads,
	}
	if l.Severity == diag.Warning {
		f.Severity = SeverityWarning
	}
	for _, r := range l.Related {
		f.Related = append(f.Related, Related{Span: spanOf(r.Loc), Note: r.Note})
	}
	for _, fr := range l.Stack {
		f.Stack = append(f.Stack, Frame{Fn: fr.Fn, Span: spanOf(fr.Loc)})
	}
	return f
}

func spanOf(l source.Location) Span {
	return Span{File: l.Path, Line: l.Line, Col: l.Col, EndLine: l.EndLine, EndCol: l.EndCol}
}

// summaryOf is the API's form of diag's counts (rules F7, F8).
func summaryOf(s diag.Summary) Summary {
	out := Summary{Errors: s.Errors, Warnings: s.Warnings, Packages: s.Packages}
	for _, t := range s.Truncated {
		out.Truncated = append(out.Truncated, Truncation{Package: t.Package, Errors: t.Errors, Warnings: t.Warnings})
	}
	return out
}
