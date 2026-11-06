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

// activeLayers are the two ways a layer file is met: active, and present but not active.
var activeLayers = [][]string{{"dev"}, nil}

// selectorKeySrc is items, holding {k}, and h, whose rr is keyed by a ref to an item.
const selectorKeySrc = "package p\n\nrecord I {\n  id: String\n}\n\nrecord R {\n  it: ref items\n  n: Int = 0\n}\n\n" +
	"record H {\n  rr: [R] keyed by it\n}\n\n" +
	"let items: [I] keyed by id = [{ id: \"{k}\" }, { id: \"bow\" }]\n\nlet h: H = { rr: [{ it: {k} }] }\n"

// API.md R7, E11, E22, X2 (DECISIONS 310, 484, 494): a line replacing h.rr[sword] whole with the key
// field `items.sword`, a selector the 310 cascade leaves as written: the segment and the selector's
// name are rewritten once each (was ErrInternal, two changes overlapped), active or not.
func TestRenameLayerLineSelectorKeyField(t *testing.T) {
	for _, form := range []struct{ name, seg string }{{"dot", "rr.{k}"}, {"bracket", "rr[{k}]"}} {
		seg := form.seg
		for _, layers := range activeLayers {
			t.Run(fmt.Sprintf("%s active %v", form.name, layers != nil), func(t *testing.T) {
				files := func(key string) map[string]string {
					layer := "package p\nlayer dev\n\namend h {\n  " + seg + ": { it: items.{k} }\n}\n"
					fill := strings.NewReplacer("{k}", key)
					return map[string]string{srcMain: formatted(t, srcMain, fill.Replace(selectorKeySrc)), sharedLayer: formatted(t, sharedLayer, fill.Replace(layer))}
				}
				src := files("sword")[srcMain]
				want := fmt.Sprintf("layer %s:5, layer %s:5, value %s:%d", sharedLayer, sharedLayer, srcMain, lineOf(src, "let h:"))
				if layers != nil { // the layered value, an entry read, is computed, at the let's name (DECISIONS 489)
					want += fmt.Sprintf(", value %s:%d", srcMain, lineOf(src, "let h:"))
				}
				sharedRow{l: sharedLevel{path: "p:items[sword]"}, layers: layers, before: files("sword"), after: files("lance"), want: want, to: "lance"}.run(t)
			})
		}
	}
}

// API.md R7, E11, E19 (DECISIONS 491, 494, 499): a line replacing h.rr[sword] whole whose key field
// states another item, `bow`, names items[bow]: it is neither listed nor rewritten, and E1913 refuses
// the rename, active or not.
func TestRenameLayerLineOtherRefKeyField(t *testing.T) {
	layer := "package p\nlayer dev\n\namend h {\n  rr.sword: { it: bow, n: 2 }\n}\n"
	before := map[string]string{srcMain: formatted(t, srcMain, strings.ReplaceAll(selectorKeySrc, "{k}", "sword")), sharedLayer: formatted(t, sharedLayer, layer)}
	want := fmt.Sprintf("layer %s:5, value %s:%d", sharedLayer, srcMain, lineOf(before[srcMain], "let h:"))
	for _, layers := range activeLayers {
		t.Run(fmt.Sprintf("active %v", layers != nil), func(t *testing.T) {
			keyMismatch{path: "p:items[sword]", key: canon.Key("lance"), layers: layers, before: before, want: want, poisoned: layers != nil}.run(t)
		})
	}
}

// constKeySrc is aa, whose oo holds an element keyed y, the constant KY and the let ky, both "y".
const constKeySrc = "package p\n\nconst KY = \"y\"\n\nrecord S {\n  k: String\n  w: Int = 0\n}\n\nrecord A {\n  oo: [S] keyed by k\n}\n\n" +
	"let ky: String = \"y\"\n\nlet aa: A = { oo: [{ k: \"{k}\" }] }\n"

// API.md R7, E11, E19 (DECISIONS 318, 464, 491, 494, 499): a line replacing aa.oo[y] whole with a key
// field that states no key as a token (a constant, a let, an expression) or states another ("z") is
// listed by its segment alone and left as written: E1913 refuses the rename; active, it is `layered`.
func TestRenameLayerLineNamedKeyField(t *testing.T) {
	fields := []struct{ name, text string }{{"constant", "KY"}, {"let", "ky"}, {"expression", "\"y\" + \"\""}, {"another key", "\"z\""}}
	for _, field := range fields {
		layer := "package p\nlayer dev\n\namend aa {\n  oo.y: { k: " + field.text + ", w: 3 }\n}\n"
		before := map[string]string{srcMain: formatted(t, srcMain, strings.ReplaceAll(constKeySrc, "{k}", "y")), sharedLayer: formatted(t, sharedLayer, layer)}
		want := fmt.Sprintf("layer %s:5", sharedLayer)
		t.Run(field.name+" active true", func(t *testing.T) {
			if field.name == "another key" {
				keyMismatch{path: "p:aa.oo[y]", key: canon.Key("q"), layers: []string{"dev"}, before: before, poisoned: true}.run(t)
				return
			}
			sharedRow{l: sharedLevel{path: "p:aa.oo[y]"}, layers: []string{"dev"}, before: before, want: want, to: "q", refused: canon.ReasonLayered}.run(t)
		})
		t.Run(field.name+" active false", func(t *testing.T) {
			keyMismatch{path: "p:aa.oo[y]", key: canon.Key("q"), before: before, want: want}.run(t)
		})
	}
}

// keyMismatch is a rename a layer line's key field, left as written, makes E1913 with layers active:
// want is its Refs; nothing is written (API.md E19; DECISIONS 499).
type keyMismatch struct {
	path     string
	key      canon.Lit
	layers   []string
	before   map[string]string
	want     string
	poisoned bool // the line gives the element another key under an active layer already
}

// run checks k's Refs, then that the rename is a RejectedError holding E1913, never an internal
// error, and writes nothing; a line that already gives the element another key under an active
// layer poisons its let (E1913 is hard), so Refs cannot compute it and is not checked.
func (k keyMismatch) run(t *testing.T) {
	t.Helper()
	p, m := sharedOpen(t, k.before, k.layers)
	ctx := context.Background()
	if !k.poisoned {
		refs, err := p.Refs(ctx, k.path)
		if err != nil {
			t.Fatalf("API.md R7: Refs: %v", err)
		}
		if got := sharedRefs(refs.Refs); got != k.want {
			t.Errorf("API.md R7 (DECISIONS 491, 494): the refs are %q, want %q", got, k.want)
		}
	}
	_, err := p.Edit(ctx, canon.Edit{Ops: []canon.Op{canon.Rename(k.path, k.key)}})
	var re *canon.RejectedError
	var ie *canon.InternalError
	if errors.As(err, &ie) || !errors.As(err, &re) || !slices.ContainsFunc(re.Findings, func(f canon.Finding) bool { return f.Code == string(diag.E1913.Def().Code) }) {
		t.Fatalf("API.md E19, X2 (DECISIONS 499): the rename gives %v, want a RejectedError holding %s", err, diag.E1913.Def().Code)
	}
	k.unwritten(t, m)
}

// unwritten checks every file of k still holds its bytes before the rename (E19).
func (k keyMismatch) unwritten(t *testing.T, m *memFS) {
	t.Helper()
	for _, name := range slices.Sorted(maps.Keys(k.before)) {
		if got := read(t, m, name); got != k.before[name] {
			t.Errorf("API.md E19: %s is written:\n%s", name, got)
		}
	}
}

// refMapSrc is items, holding {k}, mm, whose map a is keyed and valued by refs to items, and {extra}.
const refMapSrc = "package p\n\nrecord I {\n  id: String\n}\n\nrecord M {\n  a: {ref items: ref items} = {}\n}\n\n" +
	"let items: [I] keyed by id = [{ id: \"{k}\" }, { id: \"axe\" }]\n\nlet mm: M = {}\n{extra}"

// API.md R7, E11, E22 (DECISIONS 489, 494, 496): a layer line's map literal `{ bow: bow }` typed by refs
// to items states items[bow] twice: two `layer` refs, each rewritten once, active or not, as strings
// where a let lance is in scope (its Undo keeps the strings, E22 restoring values).
func TestRefsLayerLineRefMapKey(t *testing.T) {
	rows := []struct {
		name, extra, after, undone string
	}{
		{"bare", "", "a: { lance: lance }", ""},
		{"shadowed", "\nlet lance: ref items = axe\n", "a: { \"lance\": \"lance\" }", "a: { \"bow\": \"bow\" }"},
	}
	for _, r := range rows {
		for _, layers := range activeLayers {
			t.Run(fmt.Sprintf("%s active %v", r.name, layers != nil), func(t *testing.T) {
				files := func(key, line string) map[string]string {
					src := strings.NewReplacer("{k}", key, "{extra}", r.extra).Replace(refMapSrc)
					layer := "package p\nlayer dev\n\namend mm {\n  " + line + "\n}\n"
					return map[string]string{srcMain: formatted(t, srcMain, src), sharedLayer: formatted(t, sharedLayer, layer)}
				}
				row := sharedRow{l: sharedLevel{path: "p:items[bow]"}, layers: layers, before: files("bow", "a: { bow: bow }"), after: files("lance", r.after),
					want: fmt.Sprintf("layer %s:5, layer %s:5", sharedLayer, sharedLayer), to: "lance"}
				if r.undone != "" {
					row.undone = files("bow", r.undone)
				}
				row.run(t)
			})
		}
	}
}

// API.md R7, E11, E22 (DECISIONS 488, 490, 491, 494): `hs.one.rs[a]` and `hs.one.rs[Q.a]`, the member a
// keying a list by a ref whose chain ends at an enum, are `code` refs of hs[one].rs[a], beside a layer's
// segment, active or not; renaming the element to b rewrites the member name once (it was E4002).
func TestRefsRefKeyedEnumLeafRead(t *testing.T) {
	for _, form := range []struct{ name, qual string }{{"bare", ""}, {"qualified", "Q."}} {
		for _, layers := range activeLayers {
			t.Run(fmt.Sprintf("%s active %v", form.name, layers != nil), func(t *testing.T) {
				files := func(key string) map[string]string {
					src := strings.ReplaceAll(enumLeafSrc, "{k}", key) + "\nlet v: Int = hs.one.rs[" + form.qual + key + "].n\n"
					layer := "package p\nlayer dev\n\namend hs {\n  one.rs[" + key + "].n: 4\n}\n"
					return map[string]string{srcMain: formatted(t, srcMain, src), sharedLayer: formatted(t, sharedLayer, layer)}
				}
				want := fmt.Sprintf("layer %s:5, code %s:%d", sharedLayer, srcMain, lineOf(files("a")[srcMain], "let v:"))
				sharedRow{l: sharedLevel{path: "p:hs[one].rs[a]"}, layers: layers, before: files("a"), after: files("b"), want: want, key: canon.Member("b")}.run(t)
			})
		}
	}
}

// defaultMapSrc is items, holding {k}, and M, whose map's default is keyed by a ref to {k}; mm takes it.
const defaultMapSrc = "package p\n\nrecord I {\n  id: String\n}\n\nrecord M {\n  a: {ref items: Int} = { {k}: 1 }\n}\n\n" +
	"let items: [I] keyed by id = [{ id: \"{k}\" }, { id: \"axe\" }]\n\nlet mm: M = {}\n"

// API.md R7, E11, E22 (DECISIONS 483, 496): a record field default's map key `{ bow: 1 }`, typed a ref
// into the let items, is a `code` ref of items[bow] (every instance taking it names that entry), listed
// once and rewritten once, a layer present active or not (it was E3501).
func TestRefsDefaultRefMapKey(t *testing.T) {
	for _, layers := range activeLayers {
		t.Run(fmt.Sprintf("active %v", layers != nil), func(t *testing.T) {
			files := func(key string) map[string]string {
				layer := "package p\nlayer dev\n\namend mm {\n  a: { axe: 2 }\n}\n"
				return map[string]string{srcMain: formatted(t, srcMain, strings.ReplaceAll(defaultMapSrc, "{k}", key)), sharedLayer: formatted(t, sharedLayer, layer)}
			}
			want := fmt.Sprintf("code %s:%d", srcMain, lineOf(files("bow")[srcMain], "  a:"))
			sharedRow{l: sharedLevel{path: "p:items[bow]"}, layers: layers, before: files("bow"), after: files("lance"), want: want, to: "lance"}.run(t)
		})
	}
}
