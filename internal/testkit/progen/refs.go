package progen

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	canon "github.com/fantasim/canonlang/api"
)

// Ref is one reference the API lists for an enum member (API.md R7): how it names the member, the
// package holding it and its span.
type Ref struct {
	Kind    canon.RefKind
	Package string
	Span    canon.Span
}

// Refs answers reference queries on a project opened read-only through the API.
type Refs struct {
	p *canon.Project
}

// OpenRefs opens p, its roots redirected as Run's, for reference queries.
func OpenRefs(p *Project, roots map[string]string) (*Refs, error) {
	ap, err := canon.Open(projectDir, canon.Options{FS: readOnly{m: p.fsys(), volume: filepath.VolumeName}, Roots: roots})
	if err != nil {
		return nil, fmt.Errorf("progen: %w", err)
	}
	return &Refs{p: ap}, nil
}

// Member lists every reference to the enum member at path ("pkg:Enum.member", API.md P7a), in
// values, loaded data, code, views, checks and layers alike.
func (r *Refs) Member(ctx context.Context, path string) ([]Ref, error) {
	res, err := r.p.Refs(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("progen: %w", err)
	}
	out := make([]Ref, 0, len(res.Refs))
	for _, x := range res.Refs {
		out = append(out, Ref{Kind: x.Kind, Package: x.Package, Span: x.Span})
	}
	return out, nil
}

// Close releases the project the queries read.
func (r *Refs) Close() error { return r.p.Close() }

// readOnly is m refusing every write, a name's volume ("D:/p" on Windows) dropped: API.md §2.2.
type readOnly struct {
	m      memFS
	volume func(string) string
}

func (r readOnly) ReadFile(name string) ([]byte, error) { return r.m.ReadFile(r.local(name)) }

func (r readOnly) Stat(name string) (fs.FileInfo, error) { return r.m.Stat(r.local(name)) }

func (r readOnly) ReadDir(name string) ([]fs.DirEntry, error) { return r.m.ReadDir(r.local(name)) }

// EvalSymlinks is memFS's on name without its volume, the volume as name writes it put back.
func (r readOnly) EvalSymlinks(name string) (string, error) {
	n := len(r.volume(name))
	resolved, err := r.m.EvalSymlinks(name[n:])
	if err != nil {
		return "", err
	}
	return name[:n] + resolved, nil
}

// local is name without its volume: memFS holds one volume's '/' tree.
func (r readOnly) local(name string) string { return name[len(r.volume(name)):] }

func (readOnly) WriteFile(string, []byte) error { return errReadOnly }
func (readOnly) Rename(string, string) error    { return errReadOnly }
func (readOnly) Remove(string) error            { return errReadOnly }
func (readOnly) MkdirAll(string) error          { return errReadOnly }
