package eval_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"github.com/fantasim/canonlang/internal/wire"
	"golang.org/x/tools/txtar"
)

const (
	pathsFile = "paths"
	provFile  = "provenance.txt"
	keySuffix = "@key"
	rootMark  = "@"
	brackets  = "[]"
	segStart  = ".["
)

// provKinds name the provenance kinds as API.md's OriginKind does.
var provKinds = map[value.ProvKind]string{
	value.ProvLiteral: "literal", value.ProvJSON: "json", value.ProvCSV: "csv",
	value.ProvDefines: "defines", value.ProvText: "text", value.ProvDefault: "default",
	value.ProvSpread: "spread", value.ProvComputed: "computed", value.ProvLayer: "layer",
}

// EVALUATION.md §13 (EVL-07), §9.3, §11.2; CLI.md §3.7; NFR-05: each origin kind, History, inputs, two runs alike.
func TestProvenance(t *testing.T) {
	golden.Run(t, "testdata/prov/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		first := explainAll(t, c.Archive)
		if again := explainAll(t, c.Archive); again != first {
			t.Errorf("two runs differ:\n%s\n---\n%s", first, again)
		}
		return []byte(first)
	}, golden.Expected(provFile))
}

// explainAll builds an archive with its layers and explains each of its paths.
func explainAll(t *testing.T, a *txtar.Archive) string {
	t.Helper()
	p := fromArchive(t, a)
	opt := eval.Options{Layers: strings.Fields(string(archiveFile(a, layersFile)))}
	b := runBuildWith(t, p, opt, jsonLoader(t, p, a))
	var sb strings.Builder
	for _, path := range strings.Fields(string(archiveFile(a, pathsFile))) {
		sb.WriteString(b.explain(path))
	}
	return sb.String() + "\n" + b.findings(t)
}

// archiveFile is the content of the archive's file name, nil when it has none.
func archiveFile(a *txtar.Archive, name string) []byte {
	for _, f := range a.Files {
		if f.Name == name {
			return f.Data
		}
	}
	return nil
}

// jsonLoader serves load("@root/f.json") with the archive's root/f.json, decoded by wire.
func jsonLoader(t *testing.T, p *program, a *txtar.Archive) loader {
	return func(e *syntax.LoadExpr, typ types.Type) (value.Value, bool) {
		lit, ok := e.Args[0].Value.(*syntax.StringLit)
		if !ok || len(lit.Parts) != 1 {
			t.Errorf("load: want one plain string argument")
			return nil, false
		}
		name := lit.Parts[0].Text
		src, err := p.fs.Add(name, "/"+name, archiveFile(a, strings.TrimPrefix(name, rootMark)))
		if err != nil {
			t.Fatal(err)
		}
		bag := diag.NewBag(p.fs, "")
		root, err := jsonsrc.Parse(src, bag)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			return nil, false
		}
		v, decoded, err := (&wire.Decoder{Bag: bag}).Decode(context.Background(), wire.Selection{Node: root}, typ)
		if err != nil || !decoded {
			t.Errorf("%s: decoded %t, %v, %d findings", name, decoded, err, len(bag.Findings()))
		}
		return v, decoded
	}
}

// explain prints the value at path, then each provenance of its history, newest first.
func (b *build) explain(path string) string {
	pkg, rest, _ := strings.Cut(path, ":")
	rest, key := strings.CutSuffix(rest, keySuffix)
	end := strings.IndexAny(rest, segStart)
	if end < 0 {
		end = len(rest)
	}
	v, ok := b.values[eval.Root{Pkg: pkg, Name: rest[:end]}]
	if !ok {
		return path + " poisoned\n"
	}
	var f *types.Field
	chain := []value.Value{v}
	for _, seg := range segments(rest[end:]) {
		if v, f = step(v, seg, key); v == nil {
			break
		}
		chain = append(chain, v)
	}
	switch {
	case v == nil && f != nil && f.Input != nil:
		return path + " = input from env " + f.Input.Env + "\n"
	case v == nil:
		return path + " not found\n"
	}
	var sb strings.Builder
	sb.WriteString(path + " = " + v.CanonText() + "\n")
	for _, h := range b.ev.History(chain...) {
		sb.WriteString("  " + b.provText(h.Prov()) + "\n")
	}
	return sb.String()
}

// segments splits `.a[0]["k"]` into `.a`, `[0]`, `["k"]`.
func segments(s string) []string {
	var out []string
	for s != "" {
		end := strings.IndexAny(s[1:], segStart) + 1
		if s[0] == '[' {
			end = strings.Index(s, "]") + 1
		}
		if end <= 0 {
			end = len(s)
		}
		out, s = append(out, s[:end]), s[end:]
	}
	return out
}

// step reads one segment: a record field or table entry by name, a list element by index,
// a map entry (its key when key) by the key's canonical text.
func step(v value.Value, seg string, key bool) (value.Value, *types.Field) {
	name := strings.TrimPrefix(seg, ".")
	switch x := v.(type) {
	case *value.Record:
		for i, f := range verify.Fields(x.T) {
			if f.Name == name {
				return x.Fields[i], f
			}
		}
	case *value.Table:
		for _, en := range x.Entries {
			if en.Ident != nil && en.Ident.Key.S == name {
				return en, nil
			}
		}
	case *value.List:
		return elemAt(x, strings.Trim(seg, brackets)), nil
	case *value.Map:
		text := strings.Trim(seg, brackets)
		if s, err := strconv.Unquote(text); err == nil {
			text = s
		}
		for i, k := range x.Keys {
			if k.CanonText() == text {
				return pick(key, k, x.Vals[i]), nil
			}
		}
	}
	return nil, nil
}

// elemAt is a keyed list's element by key, a plain list's by index (API.md P1).
func elemAt(l *value.List, k string) value.Value {
	if std.Keyed(l) {
		for _, el := range l.Elems {
			if rec, ok := el.(*value.Record); ok && rec.Ident != nil && rec.Ident.Key.Text() == k {
				return el
			}
		}
		return nil
	}
	if i, err := strconv.Atoi(k); err == nil && i < len(l.Elems) {
		return l.Elems[i]
	}
	return nil
}

func pick(key bool, k, v value.Value) value.Value {
	if key {
		return k
	}
	return v
}

// provText is one provenance: kind, span, pointer, layer, stack, and via in parentheses.
func (b *build) provText(p *value.Prov) string {
	if p == nil {
		return "no provenance"
	}
	loc := b.prog.fs.Locate(p.Span)
	s := fmt.Sprintf("%s %s:%d:%d-%d:%d", provKinds[p.Kind], loc.Path, loc.Line, loc.Col, loc.EndLine, loc.EndCol)
	if p.Pointer != "" {
		s += " pointer " + p.Pointer
	}
	if p.Layer != "" {
		s += " layer " + p.Layer
	}
	for _, f := range p.Stack {
		at := b.prog.fs.Locate(f.Span)
		s += fmt.Sprintf(" in %s (%s:%d)", f.Fn, at.Path, at.Line)
	}
	if p.MoreFrames > 0 {
		s += fmt.Sprintf(" (%d more frames)", p.MoreFrames)
	}
	if p.Via != nil {
		s += " via (" + b.provText(p.Via) + ")"
	}
	return s
}
