package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
)

// goField is one generated field: its name, Go type and json tag.
type goField struct{ name, typ, tag string }

// structFields parses generated source and lists the fields of struct name.
func structFields(t *testing.T, src []byte, name string) []goField {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), outFile, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []goField
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != name {
			return true
		}
		for _, fl := range ts.Type.(*ast.StructType).Fields.List {
			typ := string(src[fl.Type.Pos()-1 : fl.Type.End()-1])
			out = append(out, goField{fl.Names[0].Name, typ, strings.Trim(fl.Tag.Value, "`")})
		}
		return false
	})
	return out
}

// obj is a closed object schema with the given members and extra keywords (",..." or "").
func obj(members, extra string) string {
	return `{"type":"object","additionalProperties":false,"properties":{` + members + `}` + extra + `}`
}

// schemaWith is a schema whose root has member u of definition u, and the given definitions.
func schemaWith(defs string) string {
	return `{"type":"object","additionalProperties":false,"properties":{"u":{"$ref":"#/$defs/u"}},"required":["u"],"$defs":{` + defs + `}}`
}

// VIEWMODEL.md J2: a union's struct keeps each branch's member order, the tag first; a member
// only some branches hold is optional and noted with their kinds.
func TestFlattenKeepsBranchOrder(t *testing.T) {
	src, err := generate([]byte(schemaWith(`"u":{"oneOf":[`+
		obj(`"kind":{"const":"a"},"x":{"type":"string"},"z":{"type":"string"}`, `,"required":["kind","z"]`)+`,`+
		obj(`"kind":{"const":"b"},"y":{"type":"string"},"z":{"type":"string"}`, `,"required":["kind","z"]`)+`]}`)), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := structFields(t, src, "U")
	want := []goField{
		{"Kind", "string", `json:"kind"`},
		{"X", "*string", `json:"x,omitzero"`},
		{"Y", "*string", `json:"y,omitzero"`},
		{"Z", "string", `json:"z"`},
	}
	if !slices.Equal(got, want) {
		t.Errorf("fields %v, want %v", got, want)
	}
	if !strings.Contains(string(src), "// kind: a\n") {
		t.Errorf("no kinds note:\n%s", src)
	}
}

// VIEWMODEL.md J3, J10: an optional member is a pointer only where its zero value is also a
// value; an integer and a number (or J10's decimal string) merge into a Number.
func TestFieldTypes(t *testing.T) {
	src, err := generate([]byte(schemaWith(`"u":{"oneOf":[`+
		obj(`"kind":{"const":"a"},"n":{"type":"integer","minimum":0},"m":{"oneOf":[{"type":"integer"},{"type":"string","pattern":"^-?[0-9]+$"}]},"s":{"type":"string","minLength":1},"c":{"const":true},"o":{"type":"integer","minimum":1},"v":{},"e":{"enum":[8,16]}`, "")+`,`+
		obj(`"kind":{"const":"b"},"n":{"type":"integer"},"m":{"type":"number"},"d":{"oneOf":[{"type":"number"},{"type":"string","pattern":"^-?[0-9]+$"}]},"k":{"oneOf":[{"type":"string"},{"type":"integer"}]},"b":{"type":"boolean"},"r":{"$ref":"#/$defs/textRef"}`, "")+`]},
		"textRef":{"oneOf":[{"type":"string"},`+obj(`"text":{"type":"string"}`, "")+`]}`)), nil)
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]string{}
	for _, f := range structFields(t, src, "U") {
		types[f.name] = f.typ
	}
	want := [][2]string{
		{"Kind", "string"}, {"N", "*int"}, {"M", "Number"}, {"S", "string"}, {"C", "bool"}, {"O", "int"},
		{"V", "json.RawMessage"}, {"E", "int"}, {"D", "Number"}, {"K", "Scalar"}, {"B", "*bool"}, {"R", "TextRef"},
	}
	for _, w := range want {
		if types[w[0]] != w[1] {
			t.Errorf("%s is %q, want %q", w[0], types[w[0]], w[1])
		}
	}
}

// Inline objects are named by their site (or an override), one schema met twice is one
// struct, and a union branch met through $ref is noted on the field.
func TestInlineNames(t *testing.T) {
	o := obj(`"a":{"type":"string"}`, `,"required":["a"]`)
	src, err := generate([]byte(schemaWith(`"u":`+obj(`"p":`+o+`,"q":{"type":"array","items":`+o+`},"r":{"$ref":"#/$defs/rb"}`, "")+
		`,"w":{"oneOf":[{"$ref":"#/$defs/rb"}]},"rb":`+obj(`"x":{"type":"string"}`, ""))),
		[]nameOverride{{"U.p", "Pair"}})
	if err != nil {
		t.Fatal(err)
	}
	fields := structFields(t, src, "U")
	if len(fields) != 3 || fields[0].typ != "*Pair" || fields[1].typ != "[]Pair" || fields[2].typ != "*W" {
		t.Errorf("fields %v, want *Pair, []Pair, *W", fields)
	}
	if !strings.Contains(string(src), "// #/$defs/rb only\n") {
		t.Errorf("no branch note:\n%s", src)
	}
}

// vmgen refuses, loudly, every schema its design cannot carry faithfully.
func TestRefusals(t *testing.T) {
	str := `{"type":"string"}`
	branch := func(kind, member, typ string) string {
		return obj(`"kind":{"const":"`+kind+`"},`+member+`:`+typ, "")
	}
	cases := []struct {
		name, schema string
		names        []nameOverride
		want         error
	}{
		{"type conflict", schemaWith(`"u":{"oneOf":[` + branch("a", `"x"`, str) + `,` + branch("b", `"x"`, `{"type":"boolean"}`) + `]}`), nil, errConflict},
		{"order conflict", schemaWith(`"u":{"oneOf":[` + obj(`"x":`+str+`,"y":`+str, "") + `,` + obj(`"y":`+str+`,"x":`+str, "") + `]}`), nil, errOrder},
		{"unknown member keyword", schemaWith(`"u":` + obj(`"x":{"type":"string","format":"date"}`, "")), nil, errKeyword},
		{"minProperties on a definition", schemaWith(`"u":` + obj(`"x":`+str, `,"minProperties":1`)), nil, errKeyword},
		{"unevaluatedProperties on a branch", schemaWith(`"u":{"oneOf":[` + branch("a", `"x"`, str) + `,` + obj(`"kind":{"const":"b"}`, `,"unevaluatedProperties":false`) + `]}`), nil, errKeyword},
		{"patternProperties at the root", `{"type":"object","additionalProperties":false,"properties":{},"patternProperties":{"^x":{}}}`, nil, errKeyword},
		{"not inside then", schemaWith(`"u":` + obj(`"x":`+str, `,"allOf":[{"if":{"properties":{"x":{"const":"a"}}},"then":{"not":{"required":["x"]}}}]`)), nil, errKeyword},
		{"keyword on a union", schemaWith(`"u":{"type":"object","oneOf":[` + obj(`"x":`+str, "") + `]}`), nil, errKeyword},
		{"additionalProperties true", schemaWith(`"u":{"type":"object","additionalProperties":true,"properties":{"x":` + str + `}}`), nil, errObject},
		{"additionalProperties schema", schemaWith(`"u":{"type":"object","additionalProperties":{},"properties":{"x":` + str + `}}`), nil, errObject},
		{"additionalProperties missing", schemaWith(`"u":{"type":"object","properties":{"x":` + str + `}}`), nil, errObject},
		{"array with properties", schemaWith(`"u":{"type":"array","additionalProperties":false,"properties":{"x":` + str + `}}`), nil, errObject},
		{"undeclared required", schemaWith(`"u":` + obj(`"x":`+str, `,"required":["y"]`)), nil, errRequired},
		{"undeclared if", schemaWith(`"u":` + obj(`"x":`+str, `,"allOf":[{"if":{"properties":{"y":{"const":1}}},"then":{"required":["x"]}}]`)), nil, errRequired},
		{"unresolved ref", schemaWith(`"u":{"$ref":"#/$defs/nope"}`), nil, errAlias},
		{"unresolved member ref", schemaWith(`"u":` + obj(`"x":{"$ref":"#/$defs/nope"}`, "")), nil, errRef},
		{"alias definition", schemaWith(`"u":` + obj(`"x":{"$ref":"#/$defs/a"}`, "") + `,"a":{"$ref":"#/$defs/b"},"b":` + obj("", "")), nil, errAlias},
		{"self-referential union", schemaWith(`"u":{"oneOf":[{"$ref":"#/$defs/u"}]}`), nil, errCycle},
		{"union cycle", schemaWith(`"u":{"oneOf":[{"$ref":"#/$defs/w"}]},"w":{"oneOf":[{"$ref":"#/$defs/u"}]}`), nil, errCycle},
		{"array of itself", schemaWith(`"u":` + obj(`"x":{"$ref":"#/$defs/l"}`, "") + `,"l":{"type":"array","items":{"$ref":"#/$defs/l"}}`), nil, errCycle},
		{"unreached definition", schemaWith(`"u":` + str + `,"lost":` + obj("", "")), nil, errUnreached},
		{"two unions", schemaWith(`"u":{"oneOf":[{"$ref":"#/$defs/a"}]},"w":{"oneOf":[{"$ref":"#/$defs/a"}]},"a":` + obj("", "")), nil, errOwner},
		{"stale override", schemaWith(`"u":` + str), []nameOverride{{"U.x", "X"}}, errStaleName},
		{"name taken", schemaWith(`"u":` + obj(`"v":`+obj("", "")+`,"x":{"$ref":"#/$defs/uV"}`, "") + `,"uV":` + obj(`"a":{}`, "")), nil, errName},
		{"mixed scalars", schemaWith(`"u":{"oneOf":[` + str + `,{"type":"boolean"}]}`), nil, errConflict},
		{"mixed enum", schemaWith(`"u":{"enum":["a",1]}`), nil, errConflict},
		{"fractional const", schemaWith(`"u":` + obj(`"x":{"const":1.5}`, "")), nil, errSchema},
		{"fractional enum", schemaWith(`"u":` + obj(`"x":{"enum":[1,2.5]}`, "")), nil, errSchema},
		{"duplicate member", `{"type":"object","properties":{"a":{},"a":{}}}`, nil, errDuplicate},
		{"trailing data", obj("", "") + ` {}`, nil, errTrailing},
		{"object without members", schemaWith(`"u":{"type":"object"}`), nil, errSchema},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := generate([]byte(c.schema), c.names); !errors.Is(err, c.want) {
				t.Errorf("err %v, want %v", err, c.want)
			}
		})
	}
}
