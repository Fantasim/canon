package canon_test

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// deepRowHead is aa: first's x holds q, second's x holds q and r; with optional fields a third
// element holds none. {x} and {q} mark second's x and its q, the keys a row may rename.
func deepRowHead(optional bool) string {
	coll := func(rec string) string { return "[" + rec + "] keyed by k" }
	if optional {
		coll = func(rec string) string { return "([" + rec + "] keyed by k)?" }
	}
	third := ""
	if optional {
		third = "  { id: \"third\" },\n"
	}
	return "package p\n\nrecord T {\n  k: String\n  w: Int = 0\n}\n\n" +
		"record S {\n  k: String\n  w: Int = 0\n  inn: " + coll("T") + "\n}\n\n" +
		"record A {\n  id: String\n  oo: " + coll("S") + "\n}\n\n" +
		"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"x\", inn: [{ k: \"q\" }] }] },\n" +
		"  { id: \"second\", oo: [{ k: \"{x}\", inn: [{ k: \"{q}\" }, { k: \"r\" }] }] },\n" + third + "]\n\n"
}

// deepMarks are the marks of a row's text and the key each stands for before the rename.
var deepMarks = []string{"{x}", "x", "{q}", "q"}

// deepRow is a read of second's x or its q (the entry ren marks, renamed to y) or of none,
// written in p after aa, and in more files; active layers are evaluated.
type deepRow struct {
	name     string
	optional bool
	ren      string
	extra    string
	more     map[string]string
	layers   []string
}

// fill is text with ren's mark as key and every other mark as its old key.
func (r deepRow) fill(text, key string) string {
	for i := 0; i < len(deepMarks); i += 2 {
		k := deepMarks[i+1]
		if deepMarks[i] == r.ren {
			k = key
		}
		text = strings.ReplaceAll(text, deepMarks[i], k)
	}
	return text
}

// path is the renamed entry.
func (r deepRow) path() string {
	if r.ren == "{x}" {
		return "p:aa[second].oo[x]"
	}
	return "p:aa[second].oo[x].inn[q]"
}

// files are p and the row's files, ren's key being key; p is in canonical layout.
func (r deepRow) files(t *testing.T, key string) map[string]string {
	t.Helper()
	src, err := canon.Format(srcMain, []byte(r.fill(deepRowHead(r.optional)+r.extra, key)))
	if err != nil {
		t.Fatalf("the fixture does not format: %v", err)
	}
	out := map[string]string{srcMain: string(src)}
	for _, name := range slices.Sorted(maps.Keys(r.more)) {
		out[name] = r.fill(r.more[name], key)
	}
	return out
}

// open opens editLaw with files, the row's layers active.
func (r deepRow) open(t *testing.T, files map[string]string) (*canon.Project, *memFS) {
	t.Helper()
	law := maps.Clone(editLaw)
	maps.Copy(law, files)
	opts := project(law)
	opts.Layers = r.layers
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	m, _ := opts.FS.(*memFS)
	return p, m
}

// deepChanged are the lines the rename changes in source files, outside aa's literal: its code refs.
func deepChanged(before, after map[string]string) string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(before)) {
		if strings.HasSuffix(name, ".layer.canon") {
			continue
		}
		a, b := strings.Split(before[name], "\n"), strings.Split(after[name], "\n")
		inAA := false
		for i := range min(len(a), len(b)) {
			inAA = strings.HasPrefix(a[i], "let aa") || inAA && a[i] != ""
			if a[i] != b[i] && !inAA {
				out = append(out, fmt.Sprintf("%s:%d", name, i+1))
			}
		}
	}
	return strings.Join(out, ", ")
}

// deepRows are the reads a rename must rewrite (marked) and those it must leave (unmarked).
var deepRows = []deepRow{
	{name: "level-1 spread copy", ren: "{q}", extra: "let cc: A = { ...aa.second, id: \"c\" }\n\nlet r1: T = cc.oo.x.inn.{q}\n"},
	{name: "level-2 spread copy", ren: "{q}", extra: "let s2: S = { ...aa.second.oo.x, k: \"x2\" }\n\nlet r2: T = s2.inn.{q}\n"},
	{name: "level-2 spread copy optional", optional: true, ren: "{q}", extra: "let s2: S = { ...aa.second.oo!.x, k: \"x2\" }\n\nlet r2: T? = s2.inn?.{q}\n"},
	{name: "let alias", ren: "{q}", extra: "let al: [A] keyed by id = aa\n\nlet r7: T = al.second.oo.x.inn.{q}\n"},
	{name: "let element", ren: "{q}", extra: "let sec: A = aa.second\n\nlet r8: T = sec.oo.x.inn.{q}\n"},
	{name: "let holding the level-2 list", ren: "{q}", extra: "let ol: [S] keyed by k = aa.second.oo\n\nlet r9: T = ol.x.inn.{q}\n"},
	{name: "function returning its argument", ren: "{q}", extra: "fn pick(a: A) -> A {\n  return a\n}\n\nlet bb: A = pick(aa.second)\n\nlet r4: T = bb.oo.x.inn.{q}\n"},
	{name: "static read in a lambda", ren: "{q}", extra: "let r10: [Int] = aa.map(e => aa.second.oo.x.inn.{q}.w)\n"},
	{name: "another package", ren: "{q}", extra: "let r0: T = aa.second.oo.x.inn.{q}\n", more: map[string]string{
		"q/q.canon": "package q\n\nimport p\n\nlet r15: p.T = p.aa.second.oo.x.inn.{q}\n\nlet r15b: p.T = p.aa.first.oo.x.inn.q\n",
	}},
	{name: "active layer", ren: "{q}", layers: []string{"dev"}, extra: "let r0: T = aa.second.oo.x.inn.{q}\n", more: map[string]string{
		"p/dev.layer.canon": "package p\nlayer dev\n\namend aa {\n  second.oo.x.inn.{q}.w: 9\n  first.oo.x.inn.q.w: 8\n}\n",
	}},
	{name: "unwrap at level 1", optional: true, ren: "{x}", extra: "let r0: S = aa.second.oo!.{x}\n\nlet r1: S = aa.first.oo!.x\n"},
	{name: "unwrap at level 2", optional: true, ren: "{q}", extra: "let r0: T = aa.second.oo!.x.inn!.{q}\n\nlet r1: T = aa.first.oo!.x.inn!.q\n"},
	{name: "spread overriding inn", ren: "{q}", extra: "let s3: S = { ...aa.second.oo.x, inn: [{ k: \"q\" }] }\n\nlet r3: T = s3.inn.q\n"},
	{name: "fresh function results", ren: "{q}", extra: "fn mk() -> A {\n  return { id: \"second\", oo: [{ k: \"x\", inn: [{ k: \"q\" }] }] }\n}\n\n" +
		"let mm: A = mk()\n\nlet m2: A = mk()\n\nlet r5: T = mm.oo.x.inn.q\n\nlet r6: T = m2.oo.x.inn.q\n"},
	{name: "another collection of the type", ren: "{q}", extra: "let bb2: [A] keyed by id = [{ id: \"second\", oo: [{ k: \"x\", inn: [{ k: \"q\" }] }] }]\n\nlet r6: T = bb2.second.oo.x.inn.q\n"},
	{name: "first's reads", ren: "{q}", extra: "let r1: T = aa.first.oo.x.inn.q\n\nlet r2: S = aa.first.oo.x\n"},
	{name: "third holds none", optional: true, ren: "{q}", extra: "let r2: T? = aa.third.oo?.x?.inn?.q\n"},
}

// API.md R7, E11, E22 (DECISIONS 410, 473): a deep read reaching second's entry through a spread
// copy, an alias, a let, a function returning its argument, a lambda, another package, a layer or
// `!` is rewritten; a read of another entry with the same keys is not; the Undo restores every file.
func TestEditRenameDeepRows(t *testing.T) {
	for _, r := range deepRows {
		t.Run(r.name, func(t *testing.T) {
			before, after := r.files(t, deepMarks[slices.Index(deepMarks, r.ren)+1]), r.files(t, "y")
			p, m := r.open(t, before)
			ctx := context.Background()
			refs, err := p.Refs(ctx, r.path())
			if err != nil {
				t.Fatalf("API.md R7: Refs: %v", err)
			}
			if got, want := optionalCodeRefs(refs.Refs), deepChanged(before, after); got != want {
				t.Errorf("API.md R7: the code refs of %s are %q, want %q", r.path(), got, want)
			}
			res, err := p.Edit(ctx, canon.Edit{Ops: []canon.Op{canon.Rename(r.path(), canon.Key("y"))}})
			if err != nil {
				t.Fatalf("API.md E19: the rename is refused: %v", err)
			}
			for _, name := range slices.Sorted(maps.Keys(before)) {
				if got := read(t, m, name); got != after[name] {
					t.Errorf("API.md E11, R7: %s after the rename:\n%s\nwant:\n%s", name, got, after[name])
				}
			}
			if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
				t.Fatalf("API.md E22: the Undo: %v", err)
			}
			for _, name := range slices.Sorted(maps.Keys(before)) {
				if got := read(t, m, name); got != before[name] {
					t.Errorf("API.md E22: %s after the Undo:\n%s", name, got)
				}
			}
		})
	}
}
