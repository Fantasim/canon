package ir_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// fpVector is a test vector of FINGERPRINT.md §7, read from the document itself.
type fpVector struct {
	text, sha, schema string
}

var (
	reVector = regexp.MustCompile("(?s)### Vector (\\d+):.*?```\n(.*?)```.*?SHA-256 `([0-9a-f]{64})`.*?`\\$schema`: `([^`]+)`")
	reHexID  = regexp.MustCompile(`@[0-9a-f]{8}$`)
)

func vectors(t *testing.T) map[int]fpVector {
	t.Helper()
	doc, err := os.ReadFile("../../spec/FINGERPRINT.md")
	if err != nil {
		t.Fatal(err)
	}
	out := map[int]fpVector{}
	for _, m := range reVector.FindAllStringSubmatch(string(doc), -1) {
		n, _ := strconv.Atoi(m[1])
		out[n] = fpVector{text: m[2], sha: m[3], schema: m[4]}
	}
	if len(out) != 10 {
		t.Fatalf("FINGERPRINT.md §7 holds %d vectors, want 10", len(out))
	}
	return out
}

// fpCase is the IR of a vector, built by hand as stage E would from its Canon source.
type fpCase struct {
	pkg, value string
	root       ir.TypeRef
	fns        []*ir.ExportFn
}

// FINGERPRINT.md §7: every vector's serialization, its SHA-256 and its `$schema`.
func TestFingerprintVectors(t *testing.T) {
	builds := []func() fpCase{
		vectorPotions, vectorStatuses, vectorEventConfig, vectorHeistia, vectorPlan,
		vectorSkills, vectorDeck, vectorRole, vectorFlow, vectorAdventureQuests,
	}
	all := vectors(t)
	for i, build := range builds {
		n, v := i+1, all[i+1]
		c := build()
		text, err := ir.Fingerprint(&c.root, c.fns)
		if err != nil {
			t.Errorf("vector %d: %v", n, err)
			continue
		}
		if string(text) != v.text {
			t.Errorf("vector %d:\n%s\nwant:\n%s", n, text, v.text)
		}
		if sum := sha256.Sum256(text); hex.EncodeToString(sum[:]) != v.sha {
			t.Errorf("vector %d: SHA-256 %x, want %s", n, sum, v.sha)
		}
		if id, err := ir.Schema(c.pkg, c.value, &c.root, c.fns); id != v.schema || err != nil {
			t.Errorf("vector %d: $schema %s (%v), want %s", n, id, err, v.schema)
		}
	}
}

// FINGERPRINT.md §2.2: the name part of values that are not records, variants or enums.
func TestSchemaName(t *testing.T) {
	anchors := ir.TypeRef{Kind: types.List, Elem: &ir.TypeRef{Kind: types.Int, Bits: 64, Signed: true}}
	id, err := ir.Schema("balance.parity", "anchors", &anchors, nil)
	if err != nil || !reHexID.MatchString(id) || id[:len(id)-9] != "balance.parity.anchors" {
		t.Errorf("Schema = %q, %v", id, err)
	}
	role := vectorRole().root
	nested := ir.TypeRef{Kind: types.Optional, Elem: &ir.TypeRef{Kind: types.List, Elem: &ir.TypeRef{Kind: types.Optional, Elem: &role}}}
	if id, _ := ir.Schema("teamboard", "roles", &nested, nil); id[:len(id)-9] != "sovcommon.roles.Role" {
		t.Errorf("[Role?]? is named %q", id)
	}
	twice := ir.TypeRef{Kind: types.List, Elem: &anchors}
	if id, _ := ir.Schema("p", "grid", &twice, nil); id[:len(id)-9] != "p.grid" {
		t.Errorf("[[Int]] is named %q", id)
	}
}

// FINGERPRINT.md §5: what changes the hash, and what does not.
func TestFingerprintChanges(t *testing.T) {
	base := vectorPotions()
	hash := func(c fpCase) string {
		id, err := ir.Schema(c.pkg, c.value, &c.root, c.fns)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	want := hash(base)
	potion := base.root.Elem.Named.(*ir.Record)
	edits := []struct {
		name    string
		edit    func()
		changes bool
	}{
		{"add @json(unit: ms)", func() { potion.Fields[3].Unit = types.UnitMs }, false},
		{"add an input field", func() {
			potion.Fields = append(potion.Fields, &ir.Field{Name: "key", Type: str(), Input: &types.Input{Env: "K"}})
		}, false},
		{"add a translated fn", func() {
			potion.Methods = append(potion.Methods, &ir.ExportFn{Name: "x", Kind: ir.FnTranslated, Result: boolean()})
		}, false},
		{"@json(unit: s)", func() { potion.Fields[3].Unit = types.UnitS }, true},
		{"Int → Int32", func() { potion.Fields[2].Type.Bits = 32 }, true},
		{"T → T?", func() { potion.Fields[4].Optional = true }, true},
		{"add $isX", func() { potion.Methods = append(potion.Methods, &ir.ExportFn{Name: "isX", Result: boolean()}) }, true},
	}
	for _, e := range edits {
		base = vectorPotions()
		potion = base.root.Elem.Named.(*ir.Record)
		e.edit()
		if got := hash(base); (got != want) != e.changes {
			t.Errorf("%s: %s, base %s", e.name, got, want)
		}
	}
}

// FINGERPRINT.md §8: a type with no wire form has no fingerprint.
func TestFingerprintRefusals(t *testing.T) {
	bad := []ir.TypeRef{
		{Kind: types.Range}, {Kind: types.Func}, {Kind: types.Int}, {Kind: types.List},
		{Kind: types.Record}, {Kind: types.TypeApp, Named: vectorRole().root.Named},
	}
	for _, b := range bad {
		if _, err := ir.Fingerprint(&b, nil); !errors.Is(err, ir.ErrFingerprint) {
			t.Errorf("kind %d: %v", b.Kind, err)
		}
	}
}

func str() ir.TypeRef      { return ir.TypeRef{Kind: types.String} }
func boolean() ir.TypeRef  { return ir.TypeRef{Kind: types.Bool} }
func integer() ir.TypeRef  { return ir.TypeRef{Kind: types.Int, Bits: 64, Signed: true} }
func float() ir.TypeRef    { return ir.TypeRef{Kind: types.Float, Bits: 64} }
func duration() ir.TypeRef { return ir.TypeRef{Kind: types.Duration} }
func uint8Ref() *ir.TypeRef {
	return &ir.TypeRef{Kind: types.Int, Bits: 8}
}
func refString() ir.TypeRef { s := str(); return ir.TypeRef{Kind: types.Ref, Key: &s} }
func listOf(e ir.TypeRef) ir.TypeRef {
	return ir.TypeRef{Kind: types.List, Elem: &e}
}
func mapOf(k, v ir.TypeRef) ir.TypeRef { return ir.TypeRef{Kind: types.Map, Key: &k, Elem: &v} }

func named(t ir.Type) ir.TypeRef {
	switch t.(type) {
	case *ir.Enum:
		return ir.TypeRef{Kind: types.Enum, Named: t}
	case *ir.Variant:
		return ir.TypeRef{Kind: types.Variant, Named: t}
	}
	return ir.TypeRef{Kind: types.Record, Named: t}
}

// fld is a field at one wire key, or at a key path.
func fld(t ir.TypeRef, path ...string) *ir.Field {
	return &ir.Field{Name: path[len(path)-1], WirePath: path, Type: t}
}

func opt(f *ir.Field, marker string) *ir.Field {
	f.Optional = true
	if marker != "" {
		f.NoneWire = []byte(marker)
	}
	return f
}

func unit(f *ir.Field, u types.Unit) *ir.Field {
	f.Unit = u
	return f
}

func enumOf(pkg, name string, wires ...string) *ir.Enum {
	e := &ir.Enum{Pkg: pkg, Name: name}
	for i, w := range wires {
		e.Members = append(e.Members, &ir.EnumMember{Name: w, Wire: w, Index: i})
	}
	return e
}

// withCodes gives an enum @codes(c) with codes from first, one apart.
func withCodes(e *ir.Enum, c *ir.TypeRef, first int64) *ir.Enum {
	e.Codes = c
	for i, m := range e.Members {
		m.Code = first + int64(i)
	}
	return e
}
