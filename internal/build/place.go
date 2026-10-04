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
	"github.com/fantasim/canonlang/internal/wire"
)

// place finds collisions, then files an output may not overwrite (CODEGEN.md §2.4, API.md B2).
func (r *run) place(outputs []*output, adopt []string) ([]*output, error) {
	outputs = r.collisions(outputs)
	for _, o := range outputs {
		old, err := r.p.fs.ReadFile(o.Abs)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			o.Status = StatusWritten
			continue
		case err != nil:
			return nil, displayError(o.Path, err)
		}
		o.old, o.existed = old, true
		if err := r.judge(o, adopt); err != nil {
			return nil, err
		}
	}
	return outputs, nil
}

// judge sets an existing output's status: unchanged, canon's to overwrite, adopted, or E8001; ownership is read only when the content differs (CODEGEN.md §2.4, §2.9).
func (r *run) judge(o *output, adopt []string) error {
	if bytes.Equal(o.old, o.Content) {
		o.Status = StatusUnchanged
		return nil
	}
	owned, err := r.owned(o, o.old)
	switch {
	case err != nil:
		return err
	case owned:
		o.Status = StatusWritten
	case adoptable(o.Output, adopt):
		o.Status = StatusAdopted
	default:
		diag.E8001.At(o.at, o.Path).Report(r.bags[o.Package])
	}
	return nil
}

// collisions reports two outputs on one file, letter case aside, but a runtime file (WIRE.md §8.1).
func (r *run) collisions(outputs []*output) []*output {
	first := map[string]*output{}
	kept := outputs[:0:0]
	for _, o := range outputs {
		key := strings.ToLower(o.Abs)
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

// adoptable reports an output the build may take over: a C++ header or a file of a text emit, listed (CODEGEN.md §2.4, §2.9).
func adoptable(o Output, adopt []string) bool {
	return (path.Ext(o.Path) == headerExt || o.Target == ir.TargetText) && slices.Contains(adopt, o.Path)
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
