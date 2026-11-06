package canon_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// depMapSrc is items, holding {i} and bow, and z, whose dependent map dw is keyed by a ref to an
// item: {def} is dw's default, which may state the key {s}, {z} z's fields, {more} lets beside them.
const depMapSrc = "package p\n\nrecord I {\n  id: String\n}\n\nrecord V {\n  w: Int = 0\n}\n\n" +
	"type P(k: ref items) = V\n\nrecord Z {\n  dw: {k in items: P(k)}{def}\n}\n\n" +
	"let items: [I] keyed by id = [{ id: \"{i}\" }, { id: \"bow\" }]\n{more}\nlet z: Z = {{z}}\n"

// API.md R7, E11, E22 (DECISIONS 496, 500): a dependent map's word key naming items[shield] in an amend
// line's value (`layer`, active or not) or a default (`code`) is listed and rewritten once, quoted
// where a let of the new name shadows it; the Undo then restores the key's value as a string.
func TestRefsDependentMapWordKey(t *testing.T) {
	const letLance = "\nlet lance: Int = 3\n"
	rows := []struct {
		name, def, more, z, line, to string
		quoted                       bool
		kind                         string
	}{
		{name: "layer line", z: " dw: {} ", line: "dw: { {s}: { w: 1 } }", to: "y", kind: "layer"},
		{name: "layer line shadowed", more: letLance, z: " dw: {} ", line: "dw: { {s}: { w: 1 } }", to: "lance", quoted: true, kind: "layer"},
		{name: "default", def: " = { {s}: {} }", to: "y", kind: "code"},
		{name: "default shadowed", def: " = { {s}: {} }", more: letLance, to: "lance", quoted: true, kind: "code"},
	}
	for _, r := range rows {
		for _, layers := range activeLayers {
			if r.line == "" && layers != nil {
				continue
			}
			t.Run(fmt.Sprintf("%s active %v", r.name, layers != nil), func(t *testing.T) {
				files := func(item, key string) map[string]string {
					fill := strings.NewReplacer("{def}", r.def, "{more}", r.more, "{z}", r.z, "{i}", item)
					src := strings.ReplaceAll(fill.Replace(depMapSrc), "{s}", key)
					out := map[string]string{srcMain: formatted(t, srcMain, src)}
					if r.line != "" {
						out[sharedLayer] = formatted(t, sharedLayer, "package p\nlayer dev\n\namend z {\n  "+strings.ReplaceAll(r.line, "{s}", key)+"\n}\n")
					}
					return out
				}
				newKey := r.to
				if r.quoted {
					newKey = `"` + r.to + `"`
				}
				before := files("shield", "shield")
				want := fmt.Sprintf("%s %s:5", r.kind, sharedLayer)
				if r.line == "" {
					want = fmt.Sprintf("code %s:%d", srcMain, lineOf(before[srcMain], "  dw: {k in items"))
				}
				row := sharedRow{l: sharedLevel{path: "p:items[shield]"}, layers: layers, before: before, after: files(r.to, newKey), want: want, to: r.to}
				if r.quoted {
					row.undone = files("shield", `"shield"`)
				}
				row.run(t)
			})
		}
	}
}

// mapLineSrc is aa, whose first and second each hold, in oo, x (first also y), and a map m keyed by a
// ref into their own oo, {mf} and {ms} giving each one's m, {def} m's default.
const mapLineSrc = "package p\n\nrecord S {\n  k: String\n  w: Int = 0\n}\n\n" +
	"record A {\n  id: String\n  oo: [S] keyed by k\n  m: {ref S: Int}{def}\n}\n\n" +
	"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"{f}\" }, { k: \"y\" }]{mf} },\n" +
	"  { id: \"second\", oo: [{ k: \"{s}\" }]{ms} },\n]\n"

// API.md R7, E11, E22 (DECISIONS 483, 494, 500): a map word key typed `ref S`, S each instance's own
// oo, in an amend line's value (`second.m: { x: 1 }`) is a `layer` ref of the x of the instance the
// line's path reaches only, active or not, rewritten once; the other instance's line is left.
func TestRefsLayerLineMapKeyPerInstance(t *testing.T) {
	marks := map[string]string{"first": "{f}", "second": "{s}"}
	for _, layers := range activeLayers {
		for _, lines := range [][]string{{"first"}, {"second"}, {"first", "second"}} {
			for _, of := range []string{"first", "second"} {
				t.Run(fmt.Sprintf("active %v %s rename %s", layers != nil, strings.Join(lines, "+"), of), func(t *testing.T) {
					files := func(key string) map[string]string {
						fill := strings.NewReplacer(marks[of], key, "{f}", "x", "{s}", "x", "{def}", "", "{mf}", ", m: {}", "{ms}", ", m: {}")
						layer := "package p\nlayer dev\n\namend aa {\n"
						for i, who := range lines {
							layer += fmt.Sprintf("  %s.m: { %s: %d }\n", who, marks[who], i+2)
						}
						return map[string]string{srcMain: formatted(t, srcMain, fill.Replace(mapLineSrc)), sharedLayer: formatted(t, sharedLayer, fill.Replace(layer+"}\n"))}
					}
					var want []string
					for i, who := range lines {
						if who == of {
							want = append(want, fmt.Sprintf("layer %s:%d", sharedLayer, i+5))
						}
					}
					sharedRow{l: sharedLevel{path: "p:aa[" + of + "].oo[x]"}, layers: layers, before: files("x"), after: files("z"), want: strings.Join(want, ", "), to: "z"}.run(t)
				})
			}
		}
	}
}

// mapLineDeepSrc is aa whose first and second each hold, in oo's x, an inn holding q ({s} in second's)
// and a map m keyed by a ref into that inn.
const mapLineDeepSrc = "package p\n\nrecord T {\n  k: String\n  w: Int = 0\n}\n\n" +
	"record S {\n  k: String\n  inn: [T] keyed by k\n  m: {ref T: Int}\n}\n\nrecord A {\n  id: String\n  oo: [S] keyed by k\n}\n\n" +
	"let aa: [A] keyed by id = [\n  { id: \"first\", oo: [{ k: \"x\", inn: [{ k: \"q\" }], m: {} }] },\n" +
	"  { id: \"second\", oo: [{ k: \"x\", inn: [{ k: \"{s}\" }], m: {} }] },\n]\n"

// API.md R7, E11, E22 (DECISIONS 483, 500): at level 2, a line's map key into each instance's own inn
// is a `layer` ref of the inn entry of the instance the line's path reaches only, active or not.
func TestRefsLayerLineMapKeyPerInstanceDeep(t *testing.T) {
	files := func(key string) map[string]string {
		layer := "package p\nlayer dev\n\namend aa {\n  first.oo.x.m: { q: 2 }\n  second.oo.x.m: { " + key + ": 3 }\n}\n"
		return map[string]string{srcMain: formatted(t, srcMain, strings.ReplaceAll(mapLineDeepSrc, "{s}", key)), sharedLayer: formatted(t, sharedLayer, layer)}
	}
	for _, layers := range activeLayers {
		t.Run(fmt.Sprintf("active %v", layers != nil), func(t *testing.T) {
			sharedRow{l: sharedLevel{path: "p:aa[second].oo[x].inn[q]"}, layers: layers, before: files("q"), after: files("z"), want: fmt.Sprintf("layer %s:6", sharedLayer), to: "z"}.run(t)
		})
	}
}

// API.md R7, E11, E12, E22 (DECISIONS 464, 483, 500): a line building second whole states its m's
// key `x`, a `layer` ref of second's x; inactive, the rename rewrites it with the literal's key
// field; active, the renamed entry is the layer's own and the rename is refused `layered`.
func TestRefsLayerLineMapKeyInBuiltInstance(t *testing.T) {
	files := func(key string) map[string]string {
		fill := strings.NewReplacer("{f}", "x", "{s}", key, "{def}", "", "{mf}", ", m: {}", "{ms}", ", m: {}")
		layer := "package p\nlayer dev\n\namend aa {\n  second: { id: \"second\", oo: [{ k: \"" + key + "\" }], m: { " + key + ": 1 } }\n}\n"
		return map[string]string{srcMain: formatted(t, srcMain, fill.Replace(mapLineSrc)), sharedLayer: formatted(t, sharedLayer, layer)}
	}
	want := fmt.Sprintf("layer %s:5", sharedLayer)
	t.Run("active false", func(t *testing.T) {
		sharedRow{l: sharedLevel{path: "p:aa[second].oo[x]"}, before: files("x"), after: files("z"), want: want, to: "z"}.run(t)
	})
	t.Run("active true", func(t *testing.T) {
		sharedRow{l: sharedLevel{path: "p:aa[second].oo[x]"}, layers: []string{"dev"}, before: files("x"), want: want, refused: canon.ReasonLayered}.run(t)
	})
}

// API.md R7, E11, E12, E22 (DECISIONS 483, 500, 503): a default's map key typed `ref S`, S each
// instance's own oo, is a `code` ref of second's x where second takes it, rewritten when only second
// does; when first takes it too, refused `computed`, Detail the path where it names second's x.
func TestRefsDefaultMapKeyPerInstance(t *testing.T) {
	cases := []struct {
		name          string
		first, second bool // each instance takes the default
		detail        string
	}{
		{name: "only first takes it", first: true},
		{name: "only second takes it", second: true},
		{name: "both take it", first: true, second: true, detail: "p:aa[second].m[x]"},
		{name: "neither takes it"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := func(key string) map[string]string {
				def := "x"
				if c.second {
					def = key
				}
				marks := []string{"{f}", "x", "{s}", key, "{def}", " = { " + def + ": 1 }", "{mf}", ", m: {}", "{ms}", ", m: {}"}
				if c.first {
					marks[7] = ""
				}
				if c.second {
					marks[9] = ""
				}
				return map[string]string{srcMain: formatted(t, srcMain, strings.NewReplacer(marks...).Replace(mapLineSrc))}
			}
			before := files("x")
			want := ""
			if c.second {
				want = fmt.Sprintf("code %s:%d", srcMain, lineOf(before[srcMain], "  m: {ref S"))
			}
			if c.detail == "" {
				sharedRow{l: sharedLevel{path: "p:aa[second].oo[x]"}, before: before, after: files("z"), want: want, to: "z"}.run(t)
				return
			}
			sharedRow{l: sharedLevel{path: "p:aa[second].oo[x]"}, before: before, want: want, refused: canon.ReasonComputed, to: "z"}.run(t)
			p, _ := sharedOpen(t, before, nil)
			_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Rename("p:aa[second].oo[x]", canon.Key("z"))}})
			var ne *canon.NotEditableError
			if !errors.As(err, &ne) || ne.Detail != c.detail {
				t.Errorf("API.md E12 (DECISIONS 503): the refusal is %v, want Detail %q", err, c.detail)
			}
		})
	}
}
