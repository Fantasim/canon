package cppgen_test

import (
	"errors"
	"strings"
	"testing"

	cppgen "github.com/fantasim/canonlang/internal/gen/cpp"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// typesMode turns small's emit into a types-mode one, then applies edit.
func typesMode(edit func(*ir.Package, *ir.Emit)) func(*ir.Package, *ir.Emit) {
	return func(p *ir.Package, e *ir.Emit) {
		e.Mode = ir.ModeTypes
		edit(p, e)
	}
}

// TestTypesRefusals is CODEGEN.md §2.2, §5.13: each refusal through its sentinel; what stage E refuses first is malformed.
func TestTypesRefusals(t *testing.T) {
	flag := field("flag", "flag", "", tBool)
	point := &ir.Record{Pkg: "demo", Name: "Point", Fields: []*ir.Field{field("x", "x", "", tInt)}}
	tests := []struct {
		name, words string
		edit        func(*ir.Package, *ir.Emit)
		want        error
	}{
		{"an input field, which stage E refuses (InputField)", "an input field in types mode",
			typesMode(withField(input("key", "KEY", tString, true, nil))), cppgen.ErrMalformed},
		{"a precomputed fn, which stage E refuses", "a stored export fn in types mode",
			typesMode(withMethod(&ir.ExportFn{Name: "f", Kind: ir.FnPrecomputed, Result: tInt})), cppgen.ErrMalformed},
		{"a computed default, which stage E refuses", "a computed default in types mode",
			typesMode(withField(&ir.Field{Name: "c", WirePath: []string{"c"}, Type: tInt, Computed: true})), cppgen.ErrMalformed},
		{"a record default, which stage E refuses (RecordConstant)", "value *value.Record", typesMode(func(p *ir.Package, _ *ir.Emit) {
			p.Types = append(p.Types, point)
			f := field("at", "at", "", ir.TypeRef{Kind: types.Record, Named: point})
			f.Default = &value.Record{Fields: []value.Value{num(1)}}
			thing(p).Fields = append(thing(p).Fields, f)
		}), cppgen.ErrMalformed},
		{"a dependent default, which stage E refuses (DependentType)", "a default holding a dependent value", typesMode(func(p *ir.Package, e *ir.Emit) {
			thing(p).Fields = append(thing(p).Fields, flag)
			withDependent("p", fromField("flag"))(p, e)
			fields := thing(p).Fields
			fields[len(fields)-1].Default = str("s")
		}), cppgen.ErrMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, e := small(field("a", "a", "", tInt))
			tt.edit(p, e)
			_, err := cppgen.Generate(p, e)
			if !errors.Is(err, tt.want) || err == nil || !strings.Contains(err.Error(), tt.words) {
				t.Errorf("Generate: %v, want %v with %q", err, tt.want, tt.words)
			}
		})
	}
}

// CODEGEN.md §2.2, §5.13: no value, container or loader; a public Decode per record and variant; no key checked.
func TestTypesModeShape(t *testing.T) {
	p, e := small(field("a", "a", "", tInt))
	e.Mode = ir.ModeTypes
	withValues(p, false, "things")
	inlineFold(p, "k", "A")
	files, err := cppgen.Generate(p, e)
	if err != nil {
		t.Fatal(err)
	}
	header, source := string(files[2].Content), string(files[3].Content)
	for _, absent := range []string{"class Things", "kThingsSchema", "Load", "Keys("} {
		if strings.Contains(header+source, absent) {
			t.Errorf("types mode wrote %q", absent)
		}
	}
	for _, want := range []string{"static std::optional<Thing> Decode(const nlohmann::json& v, std::string& error);", "std::optional<Thing> Thing::Decode("} {
		if !strings.Contains(header+source, want) {
			t.Errorf("types mode lacks %q", want)
		}
	}
}
