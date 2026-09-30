package build

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/lock"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// lockState is one selected package's canon.lock (LOCK.md §2.1) and its current facts (§3).
type lockState struct {
	pkg     string
	path    string // display path: the package directory, then canon.lock
	abs     string
	raw     []byte // the bytes on disk; nil when there is no file
	id      source.FileID
	file    *lock.File
	sources *lock.Sources
	whole   bool // read without E6005, so compared and updated (LOCK.md §4.5)
}

// compareLocks compares each selected package's lock with its current facts (LOCK.md §4).
func (r *run) compareLocks(ctx context.Context) error {
	for _, cp := range r.cps {
		st, err := r.readLock(ctx, cp)
		if err != nil {
			return err
		}
		if st.whole {
			st.file.Verify(st.sources, r.bags[cp.Path])
		}
		r.locks = append(r.locks, st)
	}
	return nil
}

func (r *run) readLock(ctx context.Context, cp *check.Package) (*lockState, error) {
	rel := path.Join(strings.ReplaceAll(cp.Path, qnameSep, pathSep), lockName)
	st := &lockState{pkg: cp.Path, path: rel, abs: project.Join(r.p.dir, rel), file: lock.New(cp.Path), whole: true}
	sources, err := r.sourcesOf(ctx, cp)
	if err != nil {
		return nil, err
	}
	st.sources = sources
	data, ok := r.s.locks[rel]
	if !ok {
		return st, nil
	}
	st.raw = data
	src, err := r.s.add(rel, st.abs, data)
	if err != nil {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	st.id = src.ID
	st.file, st.whole = lock.Parse(src.ID, src.Content, cp.Path, r.bags[cp.Path])
	return st, nil
}

// sourcesOf is the current facts of a package (LOCK.md §3), broken or poisoned ones skipped.
func (r *run) sourcesOf(ctx context.Context, cp *check.Package) (*lock.Sources, error) {
	s := lock.NewSources(cp.Path)
	for _, obj := range cp.Decls {
		name := cp.Path + qnameSep + obj.Name()
		var err error
		switch {
		case obj.Kind() == check.ObjTypeName && r.prog.Info.Broken[obj]:
			s.Skip(lock.KindEnum, name)
		case obj.Kind() == check.ObjTypeName:
			err = s.AddEnum(obj)
		case obj.Kind() == check.ObjLet:
			err = r.addTable(ctx, s, obj, name)
		}
		if err != nil {
			return nil, fmt.Errorf(fmtPackage, cp.Path, err)
		}
	}
	return s, nil
}

// addTable adds a stable table's facts, or skips it when it is broken or poisoned.
func (r *run) addTable(ctx context.Context, s *lock.Sources, obj check.Object, name string) error {
	if r.prog.Info.Broken[obj] {
		s.Skip(lock.KindTable, name)
		return nil
	}
	if obj.Type() == nil { // defence: only a check cut short leaves a let untyped, and analyze stops first
		return nil
	}
	if t, ok := obj.Type().Base().(*types.TableType); !ok || !t.Stable {
		return nil
	}
	v, ok := r.ev.Force(ctx, eval.Root{Pkg: obj.Pkg(), Name: obj.Name()})
	table, isTable := v.(*value.Table)
	if !ok || !isTable {
		s.Skip(lock.KindTable, name)
		return nil
	}
	return s.AddTable(name, table)
}

// update is the lock after recording the facts of a build without errors, the one compared
// left as it was; an empty lock is not created.
func (st *lockState) update() (*Lock, error) {
	file := st.fresh()
	before := lineSet(file.Format())
	if _, err := file.Update(st.sources); err != nil {
		return nil, fmt.Errorf(fmtPackage, st.pkg, err)
	}
	if st.raw == nil && len(file.Facts()) == 0 {
		return nil, nil
	}
	out := &Lock{Package: st.pkg, Path: st.path, Abs: st.abs, Content: file.Format(), Status: StatusWritten}
	if bytes.Equal(out.Content, st.raw) {
		out.Status = StatusUnchanged
	}
	for _, line := range strings.SplitAfter(string(out.Content), lineEnd) {
		if line != "" && !before[line] {
			out.Lines = append(out.Lines, strings.TrimSuffix(line, lineEnd))
		}
	}
	return out, nil
}

// fresh is the lock as read again, which an update may change: the compared one never does.
func (st *lockState) fresh() *lock.File {
	if st.raw == nil {
		return lock.New(st.pkg)
	}
	f, _ := lock.Parse(source.NoFile, st.raw, st.pkg, diag.NewBag(&source.FileSet{}, st.pkg)) // read whole before
	return f
}

// lineSet is the set of lines of a lock's text, each with its line end.
func lineSet(text []byte) map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.SplitAfter(string(text), lineEnd) {
		out[line] = true
	}
	return out
}
