package views_test

import (
	"reflect"
	"slices"
	"testing"
)

// typeDefs declares one of each definition of VIEWMODEL.md 12.3, reserved names, wire
// annotations, defaults of every kind and texts with and without letters.
const typeDefs = `package a

import studio

/// Kinds of things.
enum Kind @codes(UInt8) @json(codes) { title = 1, plain = 2, retired old = 3 }

/// 123
enum Bare { x }

/// A shape.
variant Shape {
  /// A round one.
  circle { r: Int }
  help { side: Int }
  retired gone
}

view Shape {
  circle "Round" { icon: gem, tone: info, help: "Round things" }
}

record Pt {
  x: Int
  y: Int
}

/// A thing of a kind.
record Thing(k: Kind) {
  /// The text.
  title: String = "x"
  /// 123
  n: Int = 1 + 2
  m: Int = n * 2
  kept: Int = 0 @json(path: "legacy.kept")
  opt: Int? = none @json(none: -1)
  dep: Int = 0 @deprecated("gone")
  dep2: Int = 0 @deprecated("123")
  dep3: Int = 0 @deprecated
  flag: Bool = true @json(int)
  kv: {String: Int} = {"a": 1}
  pt: Pt = { x: 1, y: 2 }
  sh: Shape = circle { r: 2 }
  big: Int = 9007199254740993
  f: Float = 0.1
  d: Duration = 90s
  mem: Kind = title
  wait: Duration = 1s @json(unit: s)
}

view Thing {
  n { help: "Enough" }
}

record Holder {
  kind: Kind
  thing: Thing(kind)
}

local record Hidden {
  x: Int
}

local record Used {
  x: Int
}

/// The holder.
let holder: Holder = { kind: plain, thing: {} }

/// A used local.
let used: [Used] = []
`

// VIEWMODEL.md §12.3, J10, J9, I18N.md K4, L6, L7: the definitions of the package.
func TestTypeDefinitions(t *testing.T) {
	m := demo(t, typeDefs, studioSrc).model(t, demoPkg)
	for _, c := range []struct{ name, want, rule string }{
		{"a.Kind", `{"kind":"enum","name":"Kind","help":"a:Kind.help","codes":"UInt8","members":[
			{"name":"title","wire":1,"index":0,"code":1,"label":"a:Kind.member.title"},
			{"name":"plain","wire":2,"index":1,"code":2,"label":"a:Kind.plain"},
			{"name":"old","wire":3,"index":2,"code":3,"retired":true,"label":"a:Kind.old"}]}`, "enum, K4, @json(codes)"},
		{"a.Bare", `{"kind":"enum","name":"Bare","help":{"text":"123"},"members":[{"name":"x","wire":"x","index":0,"label":"a:Bare.x"}]}`, "L7 neutral help"},
		{"a.Shape", `{"kind":"variant","name":"Shape","help":"a:Shape.help","tag":"kind","cases":[
			{"name":"circle","wire":"circle","label":"a:Shape.circle","help":"a:Shape.circle.help","icon":"gem","tone":"info",
				"fields":[{"name":"r","type":{"kind":"int","bits":64,"signed":true},"required":true,"wire":{"name":"r"}}]},
			{"name":"help","wire":"help","label":"a:Shape.case.help",
				"fields":[{"name":"side","type":{"kind":"int","bits":64,"signed":true},"required":true,"wire":{"name":"side"}}]},
			{"name":"gone","wire":"gone","retired":true,"label":"a:Shape.gone","fields":[]}]}`, "variant, D5, K4"},
	} {
		got := canonical(t, m.Types[c.name])
		if want := decode(t, []byte(c.want)); !reflect.DeepEqual(got, want) {
			t.Errorf("%s %s:\n got %s\nwant %s", c.rule, c.name, text(got), c.want)
		}
	}
	names := make([]string, 0, len(m.Types))
	//canon:unordered the names are sorted below
	for name := range m.Types {
		names = append(names, name)
	}
	slices.Sort(names)
	if want := []string{"a.Bare", "a.Holder", "a.Kind", "a.Pt", "a.Shape", "a.Thing", "a.Used"}; !slices.Equal(names, want) {
		t.Errorf("types = %v, want %v: public, or reachable from a public type or value", names, want)
	}
}

// VIEWMODEL.md §12.3 Field, J10, TYP-15, J13, I18N.md §3.3: each field's definition.
func TestFieldDefinitions(t *testing.T) {
	m := demo(t, typeDefs, studioSrc).model(t, demoPkg)
	fields := map[string]any{}
	for _, f := range canonical(t, m.Types["a.Thing"].Fields).([]any) {
		fields[member(f, "name").(string)] = f
	}
	int64Type := `{"kind":"int","bits":64,"signed":true}`
	for _, c := range []struct{ field, want, rule string }{
		{"title", `{"name":"title","type":{"kind":"string"},"default":"x","help":"a:Thing.field.title.help","wire":{"name":"title"}}`, "K4 field"},
		{"n", `{"name":"n","type":` + int64Type + `,"default":3,"help":"a:Thing.n.help","wire":{"name":"n"}}`, "a folded default, help property"},
		{"m", `{"name":"m","type":` + int64Type + `,"default":{"computed":true},"wire":{"name":"m"}}`, "TYP-15"},
		{"kept", `{"name":"kept","type":` + int64Type + `,"default":0,"wire":{"name":"kept","path":"legacy.kept"}}`, "wire path"},
		{"opt", `{"name":"opt","type":{"kind":"optional","of":` + int64Type + `},"default":null,"wire":{"name":"opt","none":-1}}`, "wire none"},
		{"dep", `{"name":"dep","type":` + int64Type + `,"default":0,"deprecated":"a:Thing.dep.deprecated","wire":{"name":"dep"}}`, "deprecated"},
		{"dep2", `{"name":"dep2","type":` + int64Type + `,"default":0,"deprecated":{"text":"123"},"wire":{"name":"dep2"}}`, "L7 deprecated"},
		{"dep3", `{"name":"dep3","type":` + int64Type + `,"default":0,"deprecated":{"text":""},"wire":{"name":"dep3"}}`, "12.3 no reason"},
		{"flag", `{"name":"flag","type":{"kind":"bool"},"default":true,"wire":{"name":"flag","int":true}}`, "wire int"},
		{"kv", `{"name":"kv","type":{"kind":"map","key":{"kind":"string"},"value":` + int64Type + `},"default":[["a",1]],"wire":{"name":"kv"}}`, "J10 map"},
		{"pt", `{"name":"pt","type":{"kind":"record","ref":"a.Pt"},"default":{"x":1,"y":2},"wire":{"name":"pt"}}`, "J10 record"},
		{"sh", `{"name":"sh","type":{"kind":"variant","ref":"a.Shape"},"default":{"$case":"circle","r":2},"wire":{"name":"sh"}}`, "J10 variant"},
		{"big", `{"name":"big","type":` + int64Type + `,"default":"9007199254740993","wire":{"name":"big"}}`, "J10 big integer"},
		{"f", `{"name":"f","type":{"kind":"float","bits":64},"default":0.1,"wire":{"name":"f"}}`, "J10 float"},
		{"d", `{"name":"d","type":{"kind":"duration"},"default":90000,"wire":{"name":"d"}}`, "J10 duration"},
		{"mem", `{"name":"mem","type":{"kind":"enum","ref":"a.Kind"},"default":"title","wire":{"name":"mem"}}`, "J10 enum"},
		{"wait", `{"name":"wait","type":{"kind":"duration"},"default":1000,"wire":{"name":"wait","unit":"s"}}`, "wire unit"},
	} {
		if want := decode(t, []byte(c.want)); !reflect.DeepEqual(fields[c.field], want) {
			t.Errorf("%s %s:\n got %s\nwant %s", c.rule, c.field, text(fields[c.field]), c.want)
		}
	}
	params := canonical(t, m.Types["a.Thing"].Params)
	if want := decode(t, []byte(`[{"name":"k","type":{"kind":"enum","ref":"a.Kind"}}]`)); !reflect.DeepEqual(params, want) {
		t.Errorf("params = %s, want %s", text(params), text(want))
	}
	thing := member(canonical(t, m.Types["a.Holder"].Fields[1]), "type")
	if want := decode(t, []byte(`{"kind":"record","ref":"a.Thing","bind":{"k":{"field":"kind"}}}`)); !reflect.DeepEqual(thing, want) {
		t.Errorf("Holder.thing type = %s, want %s (J13 bind)", text(thing), text(want))
	}
}

// VIEWMODEL.md J9, I18N.md K3: the studio package's catalogue holds only its Menu members (and
// unit suffixes), so its other declarations carry no key: no help, labels as neutral texts.
func TestStudioTexts(t *testing.T) {
	m := demo(t, "package a\n", studioSrc).model(t, studioPkg)
	for _, c := range []struct{ name, want string }{
		{"studio.Menu", `{"kind":"enum","name":"Menu","members":[{"name":"items","wire":"items","index":0,"label":"studio:Menu.items"}]}`},
		{"studio.Icon", `{"kind":"enum","name":"Icon","members":[{"name":"gem","wire":"gem","index":0,"label":{"text":"gem"}},
			{"name":"gear","wire":"gear","index":1,"label":{"text":"gear"}}]}`},
		{"studio.UnitSpec", `{"kind":"record","name":"UnitSpec","fields":[{"name":"suffix","type":{"kind":"string"},"default":"","wire":{"name":"suffix"}}]}`},
	} {
		got := canonical(t, m.Types[c.name])
		if want := decode(t, []byte(c.want)); !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got %s\nwant %s", c.name, text(got), c.want)
		}
	}
}

// pathRef is a ref into a collection held down a top-level let's fields (TYPES.md §10.2).
const pathRef = `package a

record Status {
  label: String
}

record Config {
  statuses: table Status
}

/// The configuration.
let config: Config = { statuses: { open { label: "Open" } } }

record Task {
  status: ref config.statuses
}

/// The tasks.
let tasks: [Task] = [{ status: open }]
`

// VIEWMODEL.md J8, J12, V2: the id of a collection held down a let's fields is its value path,
// and it validates.
func TestPathValueID(t *testing.T) {
	m := demo(t, pathRef, "").model(t, demoPkg)
	got := member(canonical(t, m.Types["a.Task"].Fields[0]), "type")
	want := decode(t, []byte(`{"kind":"ref","collection":"a:config.statuses","element":"a.Status","keyType":"string","count":1,"active":1}`))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("status type:\n got %s\nwant %s", text(got), text(want))
	}
	s := fragment(t, map[string]any{"type": "object", "additionalProperties": map[string]any{"$ref": "#/$defs/typeDef"}})
	validate(t, s, demoPkg, m.Types)
}
