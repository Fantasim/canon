package build

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/wire"
)

// place finds collisions, keeps what a plain or --only-root build writes (DECISIONS 343), drops the outputs under an absent optional root, then finds files an output may not overwrite (CODEGEN.md §2.4, API.md B2); the legacy text manifests the build deletes come last (CODEGEN.md §2.9).
func (r *run) place(outputs []*output, opt BuildOptions) ([]*output, error) {
	outputs = r.skipAbsent(r.makeRoot(consumers(r.collisions(outputs), opt.OnlyRoot), opt.OnlyRoot), opt.Check)
	own, err := r.owners(outputs)
	if err != nil {
		return nil, err
	}
	for _, o := range outputs {
		if o.remove {
			continue
		}
		old, err := r.p.fs.ReadFile(o.Abs)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			o.Status = StatusWritten
			continue
		case err != nil:
			return nil, displayError(o.Path, err)
		}
		o.old, o.existed = old, true
		if err := r.judge(o, opt.Adopt, own); err != nil {
			return nil, err
		}
	}
	return append(outputs, own.gone...), nil
}

// judge sets an existing output's status: unchanged, canon's to overwrite, adopted, or E8001; ownership is read only when the content differs (CODEGEN.md §2.4, §2.9).
func (r *run) judge(o *output, adopt []string, own *ownership) error {
	if bytes.Equal(o.old, o.Content) {
		o.Status = StatusUnchanged
		return nil
	}
	owned, err := own.owned(o, o.old)
	switch {
	case err != nil:
		return err
	case owned:
		o.Status = StatusWritten
	case adoptable(o, adopt):
		o.Status = StatusAdopted
	default:
		diag.E8001.At(o.at, o.Path).Report(r.bags[o.Package])
	}
	return nil
}

// reserved are the canon.lock and canon.outputs paths of every package directory, by lower-cased file, each with its display path: no output may be written there (CODEGEN.md §2.9).
func (r *run) reserved() map[string]string {
	out := map[string]string{}
	for _, lock := range lockPaths(r.s.names) {
		for _, rel := range []string{lock, path.Join(path.Dir(lock), outputsName)} {
			out[strings.ToLower(project.Join(r.p.dir, rel))] = rel
		}
	}
	return out
}

// collisions reports two outputs on one file, letter case aside, but a runtime file, and an output on a canon.lock or canon.outputs path (WIRE.md §8.1, CODEGEN.md §2.9).
func (r *run) collisions(outputs []*output) []*output {
	first := map[string]*output{}
	reserved := r.reserved()
	kept := outputs[:0:0]
	for _, o := range outputs {
		key := strings.ToLower(o.Abs)
		if o.remove { // a deletion is no output: a path differing in case from an output is another file (CODEGEN.md §2.9, DECISIONS 336)
			kept = append(kept, o)
			continue
		}
		if rel, clash := reserved[key]; clash && !o.listing {
			diag.E8152.At(o.at, rel, o.Path).Report(r.bags[o.Package])
			continue
		}
		prev, seen := first[key]
		switch {
		case !seen:
			first[key] = o
			kept = append(kept, o)
		case prev.Abs != o.Abs || !bytes.Equal(prev.Content, o.Content) || !runtimeFile(o.Abs):
			diag.E8152.At(o.at, prev.Path, o.Path).Report(r.bags[o.Package])
		}
	}
	return kept
}

// adoptable reports an output the build may take over: a C++ header or a file of a text emit but its canon.outputs, listed (CODEGEN.md §2.4, §2.9).
func adoptable(o *output, adopt []string) bool {
	return (path.Ext(o.Path) == headerExt || o.Target == ir.TargetText && !o.listing) && slices.Contains(adopt, o.Path)
}

// runtimeFile reports a runtime helper file, which several emits write alike (CODEGEN.md §2.3).
func runtimeFile(abs string) bool {
	return slices.ContainsFunc(runtimeFiles[:], func(name string) bool { return strings.HasSuffix(abs, pathSep+name) })
}

// marked reports a file canon generated (CODEGEN.md §2.4, VIEWMODEL.md V1).
func marked(o Output, content []byte) bool {
	switch {
	case o.Target == ir.TargetView:
		return jsonMarked(content, viewMarker)
	case path.Ext(o.Abs) == ir.JSONExt:
		return jsonMarked(content, schemaMarker)
	}
	return firstLineMatches(content, codeMarker)
}

// firstLineMatches reports content whose first line, a final `\r` aside, matches marker (CODEGEN.md §2.4, §2.9).
func firstLineMatches(content []byte, marker *regexp.Regexp) bool {
	line, _, _ := bytes.Cut(content, []byte(lineEnd))
	return marker.Match(bytes.TrimSuffix(line, []byte(carriageReturn)))
}

// firstLineIs reports content whose first line, a final `\r` aside, is line.
func firstLineIs(content []byte, line string) bool {
	first, _, _ := bytes.Cut(content, []byte(lineEnd))
	return string(bytes.TrimSuffix(first, []byte(carriageReturn))) == line
}

// jsonMarked reports a JSON object whose first member is a canon $schema (WIRE.md §8.4).
func jsonMarked(content []byte, schema *regexp.Regexp) bool {
	content = bytes.TrimPrefix(content, []byte(jsonsrc.UTF8BOM))
	if !json.Valid(content) {
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(content))
	want := []func(json.Token) bool{
		func(t json.Token) bool { return t == json.Delim(objectOpen) },
		func(t json.Token) bool { return t == wire.KeySchema },
		func(t json.Token) bool { s, ok := t.(string); return ok && schema.MatchString(s) },
	}
	for _, ok := range want {
		t, err := dec.Token()
		if err != nil || !ok(t) {
			return false
		}
	}
	return true
}
