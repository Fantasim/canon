package canon_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// e4002 is the code of an index out of range, which the fixtures' uncomputed lets give.
var e4002 = string(diag.E4002.Def().Code)

// refDetailS is p's S, keyed by name; refDetailHead adds P, whose pick refers to one.
const (
	refDetailS    = "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\n"
	refDetailHead = refDetailS + "record P {\n  pick: ref S\n}\n\n"
)

// fnRefSrc is p: tt holding x and y, and zz then aa, refs to x a function computes.
const fnRefSrc = refDetailHead + "let tt: [S] keyed by name = [{ name: \"x\" }, { name: \"y\" }]\n\n" +
	"fn first() -> ref tt {\n  return tt.x\n}\n\nlet zz: ref tt = first()\n\nlet aa: ref tt = first()\n"

// noRefDefaultSrc is p: h's oo takes its field's default, whose x nothing refers to.
const noRefDefaultSrc = refDetailS +
	"record H {\n  id: String\n  oo: [S] keyed by name = [{ name: \"x\" }]\n}\n\nlet h: H = { id: \"h\" }\n"

// twoDefaultSrc is defaultedSrc with x and y in oo's default, and pk picking y.
var twoDefaultSrc = strings.NewReplacer("= [{ name: \"x\" }]", "= [{ name: \"x\" }, { name: \"y\" }]",
	"{ pick: h.oo.x }", "{ pick: h.oo.y }").Replace(defaultedSrc)

// layerFnSrc is p: tt holding x and y, fx computing x, and pk then pk2 picking y.
const layerFnSrc = refDetailHead + "let tt: [S] keyed by name = [{ name: \"x\" }, { name: \"y\" }]\n\n" +
	"fn fx() -> ref tt {\n  return tt.x\n}\n\nlet pk: P = { pick: tt.y }\n\nlet pk2: P = { pick: tt.y }\n"

// sharedPickSrc is p: A's pick defaults to "x" in each instance's own oo, which {oo} gives; aa's
// first and second take it, and cp copies second.
const sharedPickSrc = "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n}\n\n" +
	"record A {\n  id: String\n  oo: [S] keyed by k{oo}\n  pick: ref S = \"x\"\n}\n\n" +
	"let aa: [A] keyed by id = [\n  { id: \"first\"{first} },\n  { id: \"second\"{second} },\n]\n\nlet cp: A = aa.second\n"

// refusalRow renames path to z in src, more beside it (named from the law's root), p's layers'
// bodies by name, active those of on in order, with AllowErrors when allow: the fixture's error
// findings (`CODE file:line`, only the intended ones) and the refusal wanted.
type refusalRow struct {
	name, src, path string
	more, layers    map[string]string
	on              []string
	allow           bool
	errs            []string
	want            error
}

// open is the row's project.
func (r refusalRow) open(t *testing.T) *canon.Project {
	t.Helper()
	law := maps.Clone(editLaw)
	maps.Copy(law, r.more)
	law[srcMain] = r.src
	for _, name := range slices.Sorted(maps.Keys(r.layers)) {
		law["p/"+name+".layer.canon"] = "package p\nlayer " + name + "\n\n" + r.layers[name]
	}
	opts := project(law)
	opts.Layers = r.on
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// run checks the fixture's error findings, then that the rename fails with exactly the row's error.
func (r refusalRow) run(t *testing.T) {
	t.Helper()
	p := r.open(t)
	ctx := context.Background()
	checked, err := p.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var errs []string
	for _, f := range checked.Findings {
		if f.Severity == canon.SeverityError {
			errs = append(errs, fmt.Sprintf("%s %s:%d", f.Code, f.Span.File, f.Span.Line))
		}
	}
	if !slices.Equal(errs, r.errs) {
		t.Fatalf("the fixture's errors are %q, want %q", errs, r.errs)
	}
	_, err = p.Edit(ctx, canon.Edit{Ops: []canon.Op{canon.Rename(r.path, canon.Key("z"))}, AllowErrors: r.allow})
	if got, want := refusalText(err), refusalText(r.want); got != want {
		t.Errorf("API.md E12, R7, R8: renaming %s gives\n%s, want\n%s", r.path, got, want)
	}
}

// refusalText is a rename's refusal as a row compares it: every field, findings only by whether
// a PathError carries some.
func refusalText(err error) string {
	var (
		ne *canon.NotEditableError
		pe *canon.PathError
	)
	switch {
	case errors.As(err, &ne):
		return fmt.Sprintf("%+v", *ne)
	case errors.As(err, &pe):
		return fmt.Sprintf("op %d %s %v %q findings %v", pe.Op, pe.Path, pe.Err, pe.Detail, len(pe.Findings) > 0)
	}
	return fmt.Sprint(err)
}

// refused is the NotEditableError of a rename of path, at op 0.
func refused(path string, reason canon.Reason, layer, detail string) error {
	return &canon.NotEditableError{Path: path, Reason: reason, Layer: layer, Detail: detail}
}

// dev is the layer dev of body.
func dev(body string) map[string]string { return map[string]string{"dev": body} }

// API.md E12, E21, R7, R8, W10 (DECISIONS 464, 489): a refused rename's Detail lists its target's
// refs not editable in R8 order as Refs reads them, layers active, whether the gate refuses it (a
// key in a shared default) or its refs do; `layered` names the first active layer setting one.
func TestRenameComputedDetail(t *testing.T) {
	idLine, on, computed := dev("amend h {\n  id: \"q\"\n}\n"), []string{"dev"}, canon.ReasonComputed
	hx, tx := "p:h.oo[x]", "p:tt[x]"
	rows := []refusalRow{
		{name: "defaulted field", src: defaultedSrc, path: hx, want: refused(hx, computed, "", "p:rr, p:pk.pick")},
		{name: "defaulted field, layer active", src: defaultedSrc, layers: idLine, on: on, path: hx, want: refused(hx, computed, "", "p:rr, p:pk.pick")},
		{name: "defaulted field, an active layer's line reads x", src: twoDefaultSrc, layers: dev("amend pk {\n  pick: h.oo.x\n}\n"), on: on, path: hx,
			want: refused(hx, computed, "", "p:rr, p:pk.pick")},
		{name: "defaulted field, a let not computed", src: noRefDefaultSrc + "\nlet bad: ref h.oo = [h.oo.x][5]\n", path: hx,
			errs: []string{e4002 + " p/p.canon:15"}, want: refused(hx, computed, "", "refs not listed: p:bad could not be computed")},
		{name: "function-computed refs", src: fnRefSrc, path: tx, want: refused(tx, computed, "", "p:zz, p:aa")},
		{name: "a ref only an active layer computes", src: layerFnSrc, layers: dev("amend pk {\n  pick: fx()\n}\n"), on: on, path: tx,
			want: refused(tx, canon.ReasonLayered, "dev", "p:pk.pick")},
		{name: "refs two active layers compute: the first layer in order", src: layerFnSrc, path: tx, on: []string{"one", "two"},
			layers: map[string]string{"one": "amend pk2 {\n  pick: fx()\n}\n", "two": "amend pk {\n  pick: fx()\n}\n"},
			want:   refused(tx, canon.ReasonLayered, "one", "p:pk.pick, p:pk2.pick")},
		{name: "a ref both active layers set: the layer of its final value", src: layerFnSrc, path: tx, on: []string{"one", "two"},
			layers: map[string]string{"one": "amend pk {\n  pick: fx()\n}\n", "two": "amend pk {\n  pick: fx()\n}\n"},
			want:   refused(tx, canon.ReasonLayered, "two", "p:pk.pick")},
		{name: "default with no ref", src: noRefDefaultSrc, path: hx, want: refused(hx, computed, "", "")},
		{name: "default with no ref, layer active", src: noRefDefaultSrc, layers: idLine, on: on, path: hx, want: refused(hx, computed, "", "")},
	}
	for _, r := range rows {
		t.Run(r.name, r.run)
	}
}

// API.md E12 (DECISIONS 483): the paths a shared default's token names join Detail after the refs,
// at the gate (the key in a default too) and past it, once each when a ref's path is one of them.
func TestRenameSharedDetail(t *testing.T) {
	gate := strings.NewReplacer("{oo}", " = [{ k: \"x\" }]", "{first}", "", "{second}", "").Replace(sharedPickSrc)
	past := strings.NewReplacer("{oo}", "", "{first}", ", oo: [{ k: \"x\" }, { k: \"y\" }]", "{second}", ", oo: [{ k: \"x\" }]").Replace(sharedPickSrc)
	path := "p:aa[second].oo[x]"
	rows := []refusalRow{
		{name: "at the gate", src: gate, path: path, want: refused(path, canon.ReasonComputed, "", "p:aa[second].pick, p:cp.pick")},
		{name: "past the gate", src: past, path: path, want: refused(path, canon.ReasonComputed, "", "p:aa[second].pick, p:cp.pick")},
		{name: "a ref's path also shared", src: past, layers: dev("amend cp {\n  pick: aa.second.oo.x\n}\n"), on: []string{"dev"}, path: path,
			want: refused(path, canon.ReasonLayered, "dev", "p:cp.pick, p:aa[second].pick")},
	}
	for _, r := range rows {
		t.Run(r.name, r.run)
	}
}

// API.md E12, R6, R7: a rename past the gate whose refs cannot all be listed fails ErrNoValue
// naming the root with no value as Refs does, qualified when it is not the renamed entry's
// resolved root, in its package or in another under the same name.
func TestRenameRefsNotComputed(t *testing.T) {
	tt := refDetailS + "let tt: [S] keyed by name = [{ name: \"x\" }, { name: \"y\" }]\n"
	q := map[string]string{"q/q.canon": "package q\n\nimport p\n\nlocal let tt: ref p.tt = [p.tt.x][5]\n"}
	noValue := func(path, root string) error {
		return &canon.PathError{Path: path, Err: canon.ErrNoValue, Detail: "root " + root, Findings: []canon.Finding{{}}}
	}
	rows := []refusalRow{
		{name: "another root of the package", src: tt + "\nlet bad: ref tt = [tt.x][5]\n", path: "p:tt[x]",
			errs: []string{e4002 + " p/p.canon:10"}, want: noValue("p:tt[x]", "p:bad")},
		{name: "a root of another package named as the path's", src: tt, more: q, path: "tt[x]",
			errs: []string{e4002 + " q/q.canon:5"}, want: noValue("tt[x]", "q:tt")},
		{name: "an active layer leaves a root uncomputed, AllowErrors", src: layerFnSrc, path: "p:tt[x]", on: []string{"dev"}, allow: true,
			layers: dev("amend pk {\n  pick: [tt.x][5]\n}\n\namend pk2 {\n  pick: fx()\n}\n"),
			errs:   []string{e4002 + " p/dev.layer.canon:5"}, want: noValue("p:tt[x]", "p:pk")},
	}
	for _, r := range rows {
		t.Run(r.name, r.run)
	}
}
