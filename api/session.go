package canon

import (
	"cmp"
	"errors"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// absolute is dir made absolute and '/'-separated, the form of FS names (API.md §2.2).
func absolute(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf(fmtWrap, ErrNoProject, err)
	}
	return filepath.ToSlash(abs), nil
}

// open is the project's build, or ErrClosed after Close (rule O6).
func (p *Project) open() (*build.Project, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, ErrClosed
	}
	return p.b, nil
}

func (p *Project) setRevision(rev string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rev = Revision(rev)
}

// apiError is an error of build as the API reports it (rules O1-O4, R1, R3): project.canon's
// errors as a *ProjectError with their findings, whether Open or a later call met them.
func apiError(err error) error {
	var oe *build.OpenError
	var ue *project.UnknownError
	switch {
	case errors.As(err, &oe):
		return &ProjectError{Err: projectSentinel(err), Findings: fromDiag(oe.Findings.Files, oe.Findings.List)}
	case errors.As(err, &ue) && errors.Is(err, build.ErrUnknownLayer):
		return fmt.Errorf(fmtUnknown, ErrUnknownLayer, ue.Name)
	case errors.As(err, &ue) && errors.Is(err, project.ErrMixedDirectory):
		return fmt.Errorf(fmtMixed, ErrUnknownPackage, ue.Name, project.ErrMixedDirectory)
	case errors.As(err, &ue):
		return fmt.Errorf(fmtUnknown, ErrUnknownPackage, ue.Name)
	}
	return err
}

func projectSentinel(err error) error {
	switch {
	case errors.Is(err, project.ErrNoProject):
		return ErrNoProject
	case errors.Is(err, project.ErrUnsupportedVersion):
		return ErrUnsupportedVersion
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
