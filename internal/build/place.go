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
		switch {
		case bytes.Equal(old, o.Content):
			o.Status = StatusUnchanged
		case marked(o.Output, old):
			o.Status = StatusWritten
		case adoptable(o.Path, adopt):
			o.Status = StatusAdopted
		default:
			diag.E8001.At(o.at, o.Path).Report(r.bags[o.Package])
		}
	}
	return outputs, nil
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

// adoptable reports an output the build may take over: a C++ header listed (CODEGEN.md §2.4).
func adoptable(display string, adopt []string) bool {
	return path.Ext(display) == headerExt && slices.Contains(adopt, display)
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
	line, _, _ := bytes.Cut(content, []byte(lineEnd))
	return codeMarker.Match(bytes.TrimSuffix(line, []byte(carriageReturn)))
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
