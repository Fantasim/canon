package viewgen_test

import (
	"bytes"
	"encoding/json"
	"math/rand/v2"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/api/vm"
	viewgen "github.com/fantasim/canonlang/internal/gen/view"
	"github.com/fantasim/canonlang/internal/types"
)

// maxSafeFuzzInt keeps a fuzzed bare integer inside VIEWMODEL.md J10's safe range; a value
// beyond it must be a vm.Number with Quoted true instead (gen.bigNumber).
const maxSafeFuzzInt = 1 << 52

// IMPLEMENTATION-PLAN.md 7.7's printer round trip: Write's output is valid JSON, and writing it
// again after decoding reproduces the same bytes (idempotence, robust to a raw container's
// reindenting); when nothing forced a reindent, decode(Write(m)) == m exactly.
func FuzzWriteDecode(f *testing.F) {
	f.Add(int64(1))
	f.Add(int64(0x5eed))
	f.Fuzz(func(t *testing.T, seed int64) {
		g := &gen{r: rand.New(rand.NewPCG(uint64(seed), uint64(seed)>>1|1)), strict: true}
		m := g.model()
		out, err := viewgen.Write(m)
		if err != nil {
			t.Fatalf("Write: %v", err)
		}
		if !json.Valid(out) {
			t.Fatalf("Write produced invalid JSON:\n%s", out)
		}
		got := decodeBytes(t, out)
		if g.strict && !reflect.DeepEqual(m, got) {
			t.Fatalf("decode(Write(m)) != m\n--- written ---\n%s", out)
		}
		out2, err := viewgen.Write(got)
		if err != nil {
			t.Fatalf("Write(decode(Write(m))): %v", err)
		}
		if !bytes.Equal(out, out2) {
			t.Fatalf("Write is not idempotent\n--- first ---\n%s\n--- second ---\n%s", out, out2)
		}
	})
}

// decodeBytes is Write's own output, read back strictly (VIEWMODEL.md V2).
func decodeBytes(t *testing.T, data []byte) *vm.ViewModel {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var m vm.ViewModel
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("decode Write's own output: %v\n%s", err, data)
	}
	return &m
}

// gen builds a randomized *vm.ViewModel; strict stays true unless a raw container is used,
// whose bytes Write's own reindenting changes (the idempotence check still covers it).
type gen struct {
	r      *rand.Rand
	strict bool
}

func (g *gen) model() *vm.ViewModel {
	m := &vm.ViewModel{
		Schema: "canon-vm/1", Package: g.name("pkg"), Language: "0.1",
		Requires: []string{}, Findings: []vm.Finding{},
		Types: map[string]vm.TypeDef{}, Views: map[string]vm.View{}, Values: map[string]vm.Value{},
		Usage: map[string]vm.Usage{}, Search: map[string]vm.SearchIndex{}, Assets: map[string]vm.Asset{},
		Units: map[string]vm.Unit{}, Widgets: map[string]vm.Widget{},
		I18N: vm.I18N{Source: "en", Languages: map[string]vm.Language{}},
	}
	for i := range g.r.IntN(4) {
		m.Types[g.name("t"+strconv.Itoa(i))] = g.typeDef()
	}
	for i := range g.r.IntN(3) {
		m.Views[g.name("v"+strconv.Itoa(i))] = g.view()
	}
	for i := range g.r.IntN(3) {
		m.Values[g.name("val"+strconv.Itoa(i))] = g.value()
	}
	for i := range g.r.IntN(3) {
		m.Findings = append(m.Findings, g.finding(i))
	}
	if g.r.IntN(2) == 0 {
		m.Requires = append(m.Requires, g.name("req"))
	}
	return m
}

// typeDef builds a record or, one time in three, an enum type definition (VIEWMODEL.md 12.3).
func (g *gen) typeDef() vm.TypeDef {
	if g.r.IntN(3) == 0 {
		return g.enumDef()
	}
	d := vm.TypeDef{Kind: "record", Name: g.name("T"), Fields: []vm.Field{}}
	if g.r.IntN(2) == 0 {
		d.Help = g.textRef()
	}
	for i := range g.r.IntN(4) {
		d.Fields = append(d.Fields, g.field(i))
	}
	return d
}

// enumDef builds an enum with 1-3 members, exercising vm.Scalar's quoted and unquoted forms.
func (g *gen) enumDef() vm.TypeDef {
	d := vm.TypeDef{Kind: "enum", Name: g.name("E"), Members: []vm.Member{}}
	for i := range 1 + g.r.IntN(3) {
		d.Members = append(d.Members, g.member(i))
	}
	return d
}

// member is one enum member; its wire is a quoted string half the time, else a safe-range
// integer's text, never the absent zero Scalar (Member.Wire is required).
func (g *gen) member(i int) vm.Member {
	wire := vm.Scalar{Text: strconv.FormatInt(g.r.Int64N(2*maxSafeFuzzInt)-maxSafeFuzzInt, 10)}
	if g.r.IntN(2) == 0 {
		wire = vm.Scalar{Text: g.alnum(1 + g.r.IntN(6)), Quoted: true}
	}
	return vm.Member{Name: g.name("M" + strconv.Itoa(i)), Index: i, Wire: wire, Label: g.textRef()}
}

// field builds one field, its default a json.RawMessage or absent (VIEWMODEL.md J10).
func (g *gen) field(i int) vm.Field {
	name := g.name("f" + strconv.Itoa(i))
	fd := vm.Field{Name: name, Type: vm.TypeExpr{Kind: "bool"}, Wire: vm.Wire{Name: name}}
	fd.Required = g.r.IntN(2) == 0
	if g.r.IntN(2) == 0 {
		fd.Default = g.raw(0)
	}
	return fd
}

// view is a minimal record view with one field, exercising Control's quoted big Number.
func (g *gen) view() vm.View {
	return vm.View{Kind: "record", Fields: map[string]vm.FieldView{
		g.name("fv"): {Label: g.textRef(), Control: g.control()},
	}}
}

// value is a minimal public value, its own control also holding a quoted big Number.
func (g *gen) value() vm.Value {
	return vm.Value{
		Name: g.alnum(4), Order: g.r.IntN(100), Type: vm.TypeExpr{Kind: "bool"},
		Label: g.textRef(), Control: g.control(), Editable: "none", Sources: vm.Sources{Files: []string{}},
	}
}

func (g *gen) control() vm.Control {
	return vm.Control{Kind: "number", Min: g.bigNumber(), Max: g.bigNumber()}
}

// bigNumber is a quoted decimal string beyond VIEWMODEL.md J10's safe range.
func (g *gen) bigNumber() vm.Number {
	v := g.r.Uint64() | (uint64(1) << 63)
	return vm.Number{Text: strconv.FormatUint(v, 10), Quoted: true}
}

func (g *gen) finding(i int) vm.Finding {
	return vm.Finding{Severity: "error", Code: "X" + strconv.Itoa(i), Package: "p", Message: g.message()}
}

// raw is a scalar json.RawMessage, or, one time in four, a small array or object of them
// (marking the model non-strict: Write reindents a container, VIEWMODEL.md J1).
func (g *gen) raw(depth int) json.RawMessage {
	if depth == 0 && g.r.IntN(4) == 0 {
		g.strict = false
		return g.rawContainer()
	}
	return g.leafRaw()
}

// rawContainer is a small array or object built from canonical leaves; object keys are
// index-named so they never collide (jsonsrc.Parse refuses a duplicate one).
func (g *gen) rawContainer() json.RawMessage {
	n := g.r.IntN(3)
	if g.r.IntN(2) == 0 {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = string(g.leafRaw())
		}
		return json.RawMessage("[" + strings.Join(parts, ",") + "]")
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = strconv.Quote("k"+strconv.Itoa(i)) + ":" + string(g.leafRaw())
	}
	return json.RawMessage("{" + strings.Join(parts, ",") + "}")
}

// leafRaw is null, a bool, a safe-range integer or a canonical float, or a plain string.
func (g *gen) leafRaw() json.RawMessage {
	switch g.r.IntN(5) {
	case 0:
		return json.RawMessage("null")
	case 1:
		return json.RawMessage(strconv.FormatBool(g.r.IntN(2) == 0))
	case 2:
		return json.RawMessage(strconv.FormatInt(g.r.Int64N(2*maxSafeFuzzInt)-maxSafeFuzzInt, 10))
	case 3:
		return json.RawMessage(types.FloatText(g.r.NormFloat64()*1e10, 64))
	default:
		return json.RawMessage(strconv.Quote(g.alnum(1 + g.r.IntN(5))))
	}
}

// textRef is a key reference, or a language-neutral one, never the absent zero value.
func (g *gen) textRef() vm.TextRef {
	if g.r.IntN(2) == 0 {
		return vm.TextRef{Key: g.name("k")}
	}
	text := g.message()
	return vm.TextRef{Text: &text}
}

// message is a string covering WIRE.md 7.3's escapes and its raw characters.
func (g *gen) message() string {
	const chars = "aZ\t\n\"\\<>&\u2028\u2029\u001bé"
	n := g.r.IntN(6)
	b := make([]rune, n)
	for i := range b {
		b[i] = rune(chars[g.r.IntN(len(chars))])
	}
	return string(b)
}

// alnum is a string of n letters and digits: Write never escapes any of them.
func (g *gen) alnum(n int) string {
	const alnum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = alnum[g.r.IntN(len(alnum))]
	}
	return string(b)
}

// name is prefix followed by 3 random letters, unique enough for one model's map keys.
func (g *gen) name(prefix string) string {
	return prefix + "." + g.alnum(3)
}
