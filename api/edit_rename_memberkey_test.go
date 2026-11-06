package canon_test

import (
	"fmt"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// memberForms are a member key written bare and qualified (DECISIONS 490).
var memberForms = []struct{ name, qual string }{{"bare", ""}, {"qualified", "Q."}}

// enumKeySrc is z, whose es is keyed by a member of Q, its element keyed {k}, and {extra}.
const enumKeySrc = "package p\n\nenum Q {\n  a\n  b\n  c\n}\n\nrecord E {\n  q: Q\n  w: Int = 0\n}\n\n" +
	"record Z {\n  es: [E] keyed by q\n}\n\nlet z: Z = { es: [{ q: {k} }] }\n{extra}"

// API.md R7, E11, E22 (DECISIONS 491, 494, 499): a line replacing a copy's element of a list keyed by
// an enum states its key twice, the key field bare or qualified (`Q.a`): both are `layer` refs of
// z.es[a], and the rename rewrites the member name only, the qualifier kept, active or not.
func TestRefsLayerLineMemberKeyField(t *testing.T) {
	for _, form := range memberForms {
		for _, layers := range activeLayers {
			t.Run(fmt.Sprintf("%s active %v", form.name, layers != nil), func(t *testing.T) {
				files := func(key string) map[string]string {
					src := strings.NewReplacer("{k}", key, "{extra}", "\nlet zc: Z = { ...z }\n").Replace(enumKeySrc)
					layer := "package p\nlayer dev\n\namend zc {\n  es[" + key + "]: { q: " + form.qual + key + ", w: 2 }\n}\n"
					return map[string]string{srcMain: formatted(t, srcMain, src), sharedLayer: formatted(t, sharedLayer, layer)}
				}
				sharedRow{l: sharedLevel{path: "p:z.es[a]"}, layers: layers, before: files("a"), after: files("b"),
					want: fmt.Sprintf("layer %s:5, layer %s:5", sharedLayer, sharedLayer), key: canon.Member("b")}.run(t)
			})
		}
	}
}

// API.md R7, E11, E22 (DECISIONS 483, 490): hasKey, get and find with the member a, bare or qualified,
// on a home field's list keyed by a ref whose chain ends at an enum, reaching hs[one]'s rs statically,
// are `code` refs of hs[one].rs[a]; the rename rewrites each member name once, active or not.
func TestRefsEnumLeafKeyTests(t *testing.T) {
	for _, form := range memberForms {
		for _, layers := range activeLayers {
			t.Run(fmt.Sprintf("%s active %v", form.name, layers != nil), func(t *testing.T) {
				files := func(key string) map[string]string {
					k := form.qual + key
					src := strings.ReplaceAll(enumLeafSrc, "{k}", key) + "\nlet t1: Bool = hs.one.rs.hasKey(" + k + ")\n\n" +
						"let t2: Bool = hs.one.rs.get(" + k + ") != none\n\nlet t3: Bool = hs.one.rs.find(" + k + ") != none\n"
					layer := "package p\nlayer dev\n\namend hs {\n  one.rs[" + key + "].n: 4\n}\n"
					return map[string]string{srcMain: formatted(t, srcMain, src), sharedLayer: formatted(t, sharedLayer, layer)}
				}
				src := files("a")[srcMain]
				want := fmt.Sprintf("layer %s:5, code %s:%d, code %s:%d, code %s:%d", sharedLayer, srcMain, lineOf(src, "let t1:"),
					srcMain, lineOf(src, "let t2:"), srcMain, lineOf(src, "let t3:"))
				sharedRow{l: sharedLevel{path: "p:hs[one].rs[a]"}, layers: layers, before: files("a"), after: files("b"), want: want, key: canon.Member("b")}.run(t)
			})
		}
	}
}

// API.md R7, E11, E22 (DECISIONS 483, 490): reads of z.es, keyed by an enum, by the member a bare or
// qualified, and its key test, reach z's element statically: `code` refs of z.es[a], beside a layer's
// segment; the rename rewrites each member name once, active or not.
func TestRefsEnumKeyedRead(t *testing.T) {
	for _, layers := range activeLayers {
		t.Run(fmt.Sprintf("active %v", layers != nil), func(t *testing.T) {
			files := func(key string) map[string]string {
				extra := "\nlet r1: Int = z.es[" + key + "].w\n\nlet r2: Int = z.es[Q." + key + "].w\n\nlet t1: Bool = z.es.hasKey(" + key + ")\n"
				layer := "package p\nlayer dev\n\namend z {\n  es[" + key + "].w: 4\n}\n"
				src := strings.NewReplacer("{k}", key, "{extra}", extra).Replace(enumKeySrc)
				return map[string]string{srcMain: formatted(t, srcMain, src), sharedLayer: formatted(t, sharedLayer, layer)}
			}
			src := files("a")[srcMain]
			want := fmt.Sprintf("layer %s:5, code %s:%d, code %s:%d, code %s:%d", sharedLayer, srcMain, lineOf(src, "let r1:"),
				srcMain, lineOf(src, "let r2:"), srcMain, lineOf(src, "let t1:"))
			sharedRow{l: sharedLevel{path: "p:z.es[a]"}, layers: layers, before: files("a"), after: files("b"), want: want, key: canon.Member("b")}.run(t)
		})
	}
}

// API.md R7, E11, E22, M7 (DECISIONS 490, 495, 496): renaming z.es[a] to the member c with `let c`
// in scope writes c bare, or qualified where it was, in the segment, the key field and the read: a
// value typed by an enum is printed bare, step 1 reading the member first; its Undo applies.
func TestRenameMemberKeyShadowed(t *testing.T) {
	for _, form := range memberForms {
		for _, layers := range activeLayers {
			t.Run(fmt.Sprintf("%s active %v", form.name, layers != nil), func(t *testing.T) {
				files := func(key string) map[string]string {
					extra := "\nlet c: Int = 3\n\nlet zc: Z = { ...z }\n\nlet r1: Int = z.es[" + key + "].w\n"
					src := strings.NewReplacer("{k}", key, "{extra}", extra).Replace(enumKeySrc)
					layer := "package p\nlayer dev\n\namend zc {\n  es[" + key + "]: { q: " + form.qual + key + ", w: 2 }\n}\n"
					return map[string]string{srcMain: formatted(t, srcMain, src), sharedLayer: formatted(t, sharedLayer, layer)}
				}
				want := fmt.Sprintf("layer %s:5, layer %s:5, code %s:%d", sharedLayer, sharedLayer, srcMain, lineOf(files("a")[srcMain], "let r1:"))
				sharedRow{l: sharedLevel{path: "p:z.es[a]"}, layers: layers, before: files("a"), after: files("c"), want: want, key: canon.Member("c")}.run(t)
			})
		}
	}
}
