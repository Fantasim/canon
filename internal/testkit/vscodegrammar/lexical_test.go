package vscodegrammar

import (
	"strings"
	"testing"
)

// lexicalCases pin the lexical rules of GRAMMAR.md section 2: each source line must carry the
// wanted scope on the wanted text, found in the rendered snapshot.
var lexicalCases = []struct {
	rule, src, want string
}{
	{"2.2 doc comment", "/// text", `"/// text" comment.line.documentation.canon`},
	{"2.2 four slashes", "//// text", `"//// text" comment.line.double-slash.canon`},
	{"2.2 trailing doc is a comment", "let x = 1 /// t", `"/// t" comment.line.double-slash.canon`},
	{"2.2 block comment", "/* a */ let", `"/* a */" comment.block.canon`},
	{"2.4 hex", "0x1F", `"0x1F" constant.numeric.hex.canon`},
	{"2.4 float", "1e-3", `"1e-3" constant.numeric.float.canon`},
	{"2.4 range is not a float", "1..2", `"1" constant.numeric.integer.canon`},
	{"2.5 duration", "x: 1h30m", `"1h30m" constant.numeric.duration.canon`},
	{"2.5 duration with underscore", "1m1_000ms", `"1m1_000ms" constant.numeric.duration.canon`},
	{"2.7 comment before regex", "f(/* c */ /x/)", `"/* c */" comment.block.canon`},
	{"2.7 regex after a comment", "f(/* c */ /x/)", `"/x/" string.regexp.canon`},
	{"2.6 escape", `"a\n"`, `"\\n" string.quoted.double.canon constant.character.escape.canon`},
	{"2.6 bad escape", `"a\x"`, `invalid.illegal.escape.canon`},
	{"2.6 interpolation", `"{heal:,} HP"`, `"heal" string.quoted.double.canon meta.interpolation.canon`},
	{"2.6 format spec", `"{heal:,} HP"`, `"," string.quoted.double.canon meta.interpolation.canon constant.other.format-spec.canon`},
	{"2.6 nested string", `"{a.join(", ")}"`, `"\", \"" string.quoted.double.canon meta.interpolation.canon string.quoted.double.canon`},
	{"2.6 raw string", `r"C:\{x}"`, `"r\"C:\\{x}\"" string.quoted.double.raw.canon`},
	{"2.7 regex after paren", `String(/^II_[/]+$/)`, `string.regexp.canon`},
	{"2.7 regex after comma", `f(a, /x/)`, `"/x/" string.regexp.canon`},
	{"2.7 division", "(a + b) / 2", `"/" keyword.operator.canon`},
	{"2.7 comment after paren", "f(/* c */ 1)", `"/* c */" comment.block.canon`},
	{"5.8 annotation", `@json("w", unit: s)`, `"json" meta.annotation.canon entity.name.function.decorator.canon`},
	{"4.3 member named like a keyword", "Icon.check", `"check" variable.other.property.canon`},
	{"4.2 past before a type name", "x: past GrantKind", `"past" storage.modifier.canon`},
	{"4.2 past before a list type", "x: past [E]", `"past" storage.modifier.canon`},
	{"4.2 past before ref", "x: past ref items", `"past" storage.modifier.canon`},
	{"4.2 past before table", "x: past table T", `"past" storage.modifier.canon`},
	{"4.2 past before stable", "x: past stable T", `"past" storage.modifier.canon`},
	{"4.2 past before fn", "x: past fn() -> T", `"past" storage.modifier.canon`},
	{"4.2 past before asset", "x: past asset", `"past" storage.modifier.canon`},
	{"4.2 past before match", "x: past match", `"past" storage.modifier.canon`},
	{"9.2 type name", "x: Int", `"Int" entity.name.type.canon`},
}

func TestLexicalRules(t *testing.T) {
	g, err := Load(grammarPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range lexicalCases {
		t.Run(c.rule, func(t *testing.T) {
			got, err := g.Tokenize(c.src)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got, c.want) {
				t.Errorf("%s: %q lacks %s in:\n%s", c.rule, c.src, c.want, got)
			}
		})
	}
}

// GRAMMAR.md 2.6: a multiline string spans lines and stops at its closing delimiter.
func TestMultilineString(t *testing.T) {
	g, err := Load(grammarPath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := g.Tokenize("let h = \"\"\"\n  first {x}\n  \"\"\"\nlet y")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`2:0-8 "  first " string.quoted.triple.canon`, `4:0-3 "let" storage.type.canon`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
}
