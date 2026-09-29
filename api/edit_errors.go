package canon

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/workspace"
)

// opSentinels maps edit's refusals of an operation to the API's (API.md §15).
var opSentinels = [...]sentinelMap{
	{from: edit.ErrBadOp, to: ErrBadOp},
	{from: edit.ErrKeyExists, to: ErrKeyExists},
	{from: edit.ErrStableKey, to: ErrStableKey},
	{from: edit.ErrPathCollision, to: ErrPathCollision},
	{from: edit.ErrBadPath, to: ErrBadPath},
	{from: edit.ErrNoPath, to: ErrNoPath},
	{from: edit.ErrAmbiguousPath, to: ErrAmbiguousPath},
	{from: edit.ErrNoValue, to: ErrNoValue},
}

// internalSentinels are edit's failures that only a compiler bug gives (rule X2).
var internalSentinels = [...]error{
	edit.ErrInternal, edit.ErrNoHost, edit.ErrNoProject, edit.ErrNotAnalyzed, edit.ErrForeign, edit.ErrRevision,
	edit.ErrChanges,
}

// editError is an edit's or a draft's failure as the API reports it (rules X1, X2, S11):
// a refusal with its op's index and path as given, an overlay (S12), a layout (M9), a stale base
// or file (S5, N9), a state forbidding writes (ErrProject), a compiler bug; else as it is.
func (p *Project) editError(ctx context.Context, err error) error {
	var (
		oe *edit.OpError
		ov *workspace.OverlayError
		nc *workspace.NotCanonicalError
	)
	switch {
	case errors.Is(err, edit.ErrJournal), errors.Is(err, edit.ErrUnwritable):
		return journalError(err) // before the cancellation: the state stays whatever the ctx
	case ctx.Err() != nil:
		return ctx.Err()
	case errors.As(err, &oe):
		return p.opError(ctx, oe.Index, oe.Path, oe.Err)
	case errors.As(err, &ov):
		return &PathError{Op: -1, Path: ov.File, Err: ErrOverlay}
	case errors.As(err, &nc):
		return &NotCanonicalError{Files: slices.Clone(nc.Files)}
	}
	return p.opError(ctx, -1, "", err)
}

// opError is edit's refusal of operation op, at path as given (-1 and "" for none), as the API's.
func (p *Project) opError(ctx context.Context, op int, path string, err error) error {
	var (
		pe *edit.PathError
		ve *edit.ValueError
		ne *edit.NotEditableError
		ce *edit.CollisionError
	)
	switch {
	case errors.As(err, &pe):
		return p.pathError(ctx, op, path, err, pe)
	case errors.Is(err, edit.ErrBadPath):
		return withOp(syntaxError(path, err, 0), op)
	case errors.As(err, &ve):
		return &ValueError{Op: op, Path: path, Expected: ve.Expected, Got: ve.Got, Detail: ve.Detail}
	case errors.As(err, &ne):
		return &NotEditableError{Op: op, Path: path, Reason: reasons[ne.Reason], Origin: ne.Origin, Layer: ne.Layer, Detail: notEditableDetail(ne)}
	case errors.As(err, &ce):
		return &PathError{Op: op, Path: path, Err: ErrPathCollision, Detail: strings.Join(ce.Paths, textListSep)}
	}
	return sentinelError(op, path, err)
}

// sentinelError is a refusal edit gave as a sentinel alone, as the API's error; any other error
// is apiError's: a stale base or file, a compiler bug, a failure of the file system as it is.
func sentinelError(op int, path string, err error) error {
	switch {
	case slices.ContainsFunc(internalSentinels[:], func(s error) bool { return errors.Is(err, s) }):
		return internalError(err)
	case errors.Is(err, edit.ErrBadValue):
		return &ValueError{Op: op, Path: path}
	case errors.Is(err, edit.ErrNotEditable):
		return &NotEditableError{Op: op, Path: path}
	}
	if i := slices.IndexFunc(opSentinels[:], func(m sentinelMap) bool { return errors.Is(err, m.from) }); i >= 0 {
		return &PathError{Op: op, Path: path, Err: opSentinels[i].to}
	}
	return apiError(err)
}

// pathError is a path edit could not resolve, as Value reports it (rules R5, R6), with op's index;
// a value that was not computed is explained by the analysis Apply read it in, never by reading
// the project again (log-2026-09-29 M4 U5b-r).
func (p *Project) pathError(ctx context.Context, op int, path string, err error, pe *edit.PathError) error {
	parsed, perr := edit.Parse(path)
	if perr != nil {
		return internalError(perr)
	}
	s := &snapshot{a: pe.At, editLayer: p.editLayer, layers: p.layers, ctx: ctx}
	return withOp(s.resolveError(path, parsed, err), op)
}

// notEditableDetail names what makes a value not editable: the references a rename cannot
// change (E12), or the hidden path holding it (log-2026-09-29 M4 U5b-r3).
func notEditableDetail(ne *edit.NotEditableError) string {
	if ne.File != "" {
		return ne.File
	}
	return strings.Join(ne.Refs, textListSep)
}

// withOp is err with op's index when it is a *PathError.
func withOp(err error, op int) error {
	var pe *PathError
	if errors.As(err, &pe) {
		pe.Op = op
	}
	return err
}
