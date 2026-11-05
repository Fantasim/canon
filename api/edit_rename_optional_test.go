package canon_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// optionalKeyed and optionalTable are a keyed list and a table literal, %s the key of the entry renamed.
const (
	optionalKeyed = `[{ name: "%s" }, { name: "z" }]`
	optionalTable = `{ %s { name: "X" }, z { name: "Z" } }`
)

// optionalRenameCase is a keyed list or table a record field holds, optional or not: its type,
// its literal (%s is the renamed entry's key), the read operator and the type a read has.
type optionalRenameCase struct {
	name, typ, list, dot, read string
}

// optionalRenameSrc is p for c, with second's entry keyed k and read as k: aa's entries first and
// second each hold an entry x, and second one z, read through oo as values.
func optionalRenameSrc(c optionalRenameCase, k string) string {
	return "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\n" +
		"record A {\n  id: String\n  oo: " + c.typ + "\n}\n\n" +
		"let aa: [A] keyed by id = [\n  { id: \"first\", oo: " + fmt.Sprintf(c.list, "x") + " },\n" +
		"  { id: \"second\", oo: " + fmt.Sprintf(c.list, k) + " },\n]\n\n" +
		"let wo: " + c.read + " = aa.second.oo" + c.dot + k + "\n\n" +
		"let wz: " + c.read + " = aa.second.oo" + c.dot + "z\n\n" +
		"let wf: " + c.read + " = aa.first.oo" + c.dot + "x\n"
}

// optionalRenameLayer amends second's entry keyed k and first's x.
func optionalRenameLayer(k string) string {
	return "package p\nlayer dev\n\namend aa {\n  second.oo." + k + ".w: 1\n  first.oo.x.w: 2\n}\n"
}

// API.md R7, E11, E19, E22 (DECISIONS 470): renaming second's x through an optional field rewrites
// its element, `aa.second.oo?.x` and `second.oo.x.w` as through a plain one, leaving z and first's
// x; the Undo restores both files.
func TestEditRenameThroughOptionalField(t *testing.T) {
	for _, c := range []optionalRenameCase{
		{"plain keyed list", "[S] keyed by name", optionalKeyed, ".", "S"},
		{"optional keyed list", "([S] keyed by name)?", optionalKeyed, "?.", "S?"},
		{"plain table", "table S", optionalTable, ".", "S"},
		{"optional table", "(table S)?", optionalTable, "?.", "S?"},
	} {
		t.Run(c.name, func(t *testing.T) {
			src, layer := optionalRenameSrc(c, "x"), optionalRenameLayer("x")
			p, m := openEdit(t, srcLaw(src, "p/dev.layer.canon", layer))
			ctx := context.Background()
			refs, err := p.Refs(ctx, "p:aa[second].oo[x]")
			if err != nil {
				t.Fatalf("API.md R7: Refs: %v", err)
			}
			if got := optionalCodeRefs(refs.Refs); got != "p/p.canon:18" {
				t.Errorf("API.md R7: the code refs of second's x are %q, want its value form only", got)
			}
			res, err := p.Edit(ctx, canon.Edit{Ops: []canon.Op{canon.Rename("p:aa[second].oo[x]", canon.Key("y"))}})
			if err != nil {
				t.Fatalf("API.md E19: the rename is refused: %v", err)
			}
			if got, want := read(t, m, srcMain), optionalRenameSrc(c, "y"); got != want {
				t.Errorf("API.md E11, R7: %s after the rename:\n%s\nwant:\n%s", srcMain, got, want)
			}
			if got, want := read(t, m, "p/dev.layer.canon"), optionalRenameLayer("y"); got != want {
				t.Errorf("API.md E11, R7: the layer after the rename:\n%s\nwant:\n%s", got, want)
			}
			if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
				t.Fatalf("API.md E22: the Undo: %v", err)
			}
			if got := read(t, m, srcMain); got != src {
				t.Errorf("API.md E22: %s after the Undo:\n%s", srcMain, got)
			}
			if got := read(t, m, "p/dev.layer.canon"); got != layer {
				t.Errorf("API.md E22: the layer after the Undo:\n%s", got)
			}
		})
	}
}

// optionalShapesSrc is p for c: holder.p, an applied record, holds entries keyed h and z; two holds
// entries keyed k and z, and three is two spread; each x is read as a value.
func optionalShapesSrc(c optionalRenameCase, h, k string) string {
	return "package p\n\nrecord S {\n  name: String\n  w: Int = 0\n}\n\nrecord Ev {\n  max: Int = 3\n}\n\n" +
		"record P(e: Ev) {\n  oo: " + c.typ + "\n}\n\nrecord Holder {\n  ev: Ev = {}\n  p: P(ev)\n}\n\n" +
		"record A {\n  id: String\n  oo: " + c.typ + "\n}\n\n" +
		"let holder: Holder = { p: { oo: " + fmt.Sprintf(c.list, h) + " } }\n\n" +
		"let two: A = { id: \"two\", oo: " + fmt.Sprintf(c.list, k) + " }\n\n" +
		"let three: A = { ...two, id: \"h\" }\n\n" +
		"let hx: " + c.read + " = holder.p.oo" + c.dot + h + "\n\n" +
		"let tx: " + c.read + " = two.oo" + c.dot + k + "\n\n" +
		"let sx: " + c.read + " = three.oo" + c.dot + k + "\n"
}

// API.md R7, E11, E19, DECISIONS 410: through an applied record's field, and through a spread
// copy holding the source's entries, an optional field's lookup is renamed as a plain one's.
func TestEditRenameOptionalShapes(t *testing.T) {
	for _, c := range []optionalRenameCase{
		{"plain keyed list", "[S] keyed by name", optionalKeyed, ".", "S"},
		{"optional keyed list", "([S] keyed by name)?", optionalKeyed, "?.", "S?"},
		{"plain table", "table S", optionalTable, ".", "S"},
		{"optional table", "(table S)?", optionalTable, "?.", "S?"},
	} {
		for _, r := range []struct{ path, h, k string }{{"p:holder.p.oo[x]", "y", "x"}, {"p:two.oo[x]", "x", "y"}} {
			t.Run(c.name+" "+r.path, func(t *testing.T) {
				p, m := openEdit(t, srcLaw(optionalShapesSrc(c, "x", "x")))
				if _, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Rename(r.path, canon.Key("y"))}}); err != nil {
					t.Fatalf("API.md E19: the rename is refused: %v", err)
				}
				if got, want := read(t, m, srcMain), optionalShapesSrc(c, r.h, r.k); got != want {
					t.Errorf("API.md E11, R7: %s after the rename:\n%s\nwant:\n%s", srcMain, got, want)
				}
			})
		}
	}
}

// API.md R11, P10, DECISIONS 470: an element or entry an optional field holds is one node at its path, as through a plain field.
func TestGraphThroughOptionalField(t *testing.T) {
	const elems = "element p:aa[first], element p:aa[first].oo[x], element p:aa[first].oo[z], element p:aa[second], element p:aa[second].oo[x], element p:aa[second].oo[z]"
	const entries = "element p:aa[first], entry p:aa[first].oo.x, entry p:aa[first].oo.z, element p:aa[second], entry p:aa[second].oo.x, entry p:aa[second].oo.z"
	for _, c := range []struct {
		optionalRenameCase
		want string
	}{
		{optionalRenameCase{"plain keyed list", "[S] keyed by name", optionalKeyed, ".", "S"}, elems},
		{optionalRenameCase{"optional keyed list", "([S] keyed by name)?", optionalKeyed, "?.", "S?"}, elems},
		{optionalRenameCase{"plain table", "table S", optionalTable, ".", "S"}, entries},
		{optionalRenameCase{"optional table", "(table S)?", optionalTable, "?.", "S?"}, entries},
	} {
		p, _ := openEdit(t, srcLaw(optionalRenameSrc(c.optionalRenameCase, "x")))
		nodes, _ := graphOf(t, p)
		var got []string
		for _, n := range nodes {
			if n.Package == "p" && n.Kind != canon.NodeRoot {
				got = append(got, fmt.Sprintf("%s %s", n.Kind, n.ID))
			}
		}
		if strings.Join(got, ", ") != c.want {
			t.Errorf("API.md R11: %s: nodes %q, want %q", c.name, strings.Join(got, ", "), c.want)
		}
	}
}

// optionalCodeRefs is the file and line of each code ref, in order.
func optionalCodeRefs(refs []canon.Ref) string {
	var out []string
	for _, r := range refs {
		if r.Kind == canon.RefCode {
			out = append(out, fmt.Sprintf("%s:%d", r.File, r.Line))
		}
	}
	return strings.Join(out, ", ")
}
