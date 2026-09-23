package diag

import (
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/source"
)

type sampleTypeArg string

func (t sampleTypeArg) String() string { return string(t) }

type sampleValueArg string

func (v sampleValueArg) CanonText() string { return string(v) }

// The sample of each argument type (ERRORS.md §1.3), used by the generated constructed table.
var (
	sampleSpan             = source.Span{File: 1, Start: 3, End: 9}
	sampleName             = "resource.vocab.items"
	sampleNames            = []string{"go", "cpp"}
	sampleChain            = []string{"a", "b", "a"}
	sampleType    TypeArg  = sampleTypeArg("Int")
	sampleTypes            = []TypeArg{sampleTypeArg("Int"), sampleTypeArg("String")}
	sampleValue   ValueArg = sampleValueArg(`"II_SYS_SYS_SCR_FARM3"`)
	sampleExpr             = source.Span{File: 1, Start: 10, End: 14}
	sampleInt     int64    = -42
	sampleRune             = '\x1f'
	samplePath             = "@resource/Server/Item/propItem.json"
	sampleLoc              = source.Span{File: 2, Start: 0, End: 1}
	samplePointer          = "/modelTypes/3"
	sampleText             = "0.9"
	sampleMessage          = E1004.At(sampleSpan).Message()
)

// goTypes is the Go parameter type of each argument type (ERRORS.md §1.3).
var goTypes = map[ArgType]reflect.Type{
	ArgTypeName:    reflect.TypeFor[string](),
	ArgTypeNames:   reflect.TypeFor[[]string](),
	ArgTypeChain:   reflect.TypeFor[[]string](),
	ArgTypeType:    reflect.TypeFor[sampleTypeArg](),
	ArgTypeTypes:   reflect.TypeFor[[]TypeArg](),
	ArgTypeValue:   reflect.TypeFor[sampleValueArg](),
	ArgTypeExpr:    reflect.TypeFor[source.Span](),
	ArgTypeInt:     reflect.TypeFor[int64](),
	ArgTypeRune:    reflect.TypeFor[rune](),
	ArgTypePath:    reflect.TypeFor[string](),
	ArgTypeLoc:     reflect.TypeFor[source.Span](),
	ArgTypePointer: reflect.TypeFor[string](),
	ArgTypeKind:    reflect.TypeFor[Kind](),
	ArgTypeText:    reflect.TypeFor[string](),
	ArgTypeMessage: reflect.TypeFor[Message](),
}

// ERRORS.md §2.2: each variant's constructor records its code, variant, span and typed arguments.
func TestEveryConstructorBuildsItsVariant(t *testing.T) {
	next := 0
	for i := range Registry {
		d := &Registry[i]
		if d.Severity == Runtime {
			continue
		}
		for v, variant := range d.Variants {
			if next == len(constructed) {
				t.Fatalf("%s: no constructed builder left", d.Code)
			}
			checkBuilder(t, constructed[next], d, v, variant)
			next++
		}
	}
	if next != len(constructed) {
		t.Errorf("%d constructed builders, %d reported messages", len(constructed), next)
	}
}

func checkBuilder(t *testing.T, b *Builder, d *Def, v int, variant Variant) {
	t.Helper()
	if b.def != d || b.variant != v || b.span != sampleSpan {
		t.Fatalf("%s variant %d: built %s variant %d", d.Code, v, b.def.Code, b.variant)
	}
	if len(b.args) != len(variant.Args) {
		t.Fatalf("%s variant %d: %d arguments, want %d", d.Code, v, len(b.args), len(variant.Args))
	}
	for i, a := range variant.Args {
		if got := reflect.TypeOf(b.args[i]); got != goTypes[a.Type] {
			t.Errorf("%s variant %d: argument %s is a %v, want a %v", d.Code, v, a.Name, got, goTypes[a.Type])
		}
	}
}

// ERRORS.md §1.4: every Kind has a name and a word; out of range gives neither.
func TestKindWords(t *testing.T) {
	if KindField.String() != "Field" || KindField.Word() != "field" || KindArray.Word() != "an array" {
		t.Errorf("KindField = %q %q, KindArray = %q", KindField.String(), KindField.Word(), KindArray.Word())
	}
	last := KindWidget + 1
	if last.String() != "" || last.Word() != "" || ArgType(255).String() != "" || Severity(3).String() != "" {
		t.Error("an out-of-range enum value has a name")
	}
}

// ERRORS.md §2.2: a Message keeps the finding it was made from, unreported.
func TestMessageKeepsItsFinding(t *testing.T) {
	b := E3501.At(sampleSpan, sampleValue, sampleName)
	if m := b.Message(); m.finding != b {
		t.Error("Message does not hold its builder")
	}
}
