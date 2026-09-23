package lock

import (
	"cmp"
	"slices"
	"strconv"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// Kind is what a fact locks (LOCK.md §1); the order of the constants is the canonical order.
type Kind uint8

// String is the kind as canon.lock writes it.
func (k Kind) String() string {
	if int(k) >= len(kindNames) {
		return ""
	}
	return kindNames[k]
}

// Value is a locked value: an integer, or a string when IsString (LOCK.md §2.2).
type Value struct {
	IsString bool
	Int      int64
	Str      string
}

// Fact is one line of canon.lock (LOCK.md §2.2); Span is the line read, zero when new.
type Fact struct {
	Kind    Kind
	Name    string // the table's let or the enum, qualified
	Field   string // KindField only
	Value   Value
	Holder  string
	Retired bool
	Span    source.Span
}

// File is the set of facts of one package's lock (LOCK.md §2.4), in canonical order.
type File struct {
	Package string
	facts   []Fact
}

// New is the empty lock of package pkg.
func New(pkg string) *File {
	return &File{Package: pkg}
}

// Facts is every fact, in canonical order (LOCK.md §2.3).
func (f *File) Facts() []Fact {
	return slices.Clone(f.facts)
}

// Add merges a fact into the set and reports whether it changed. A fact whose line Parse
// would refuse or read as another fact is refused (ErrBadFact), the set unchanged.
func (f *File) Add(fact Fact) (bool, error) {
	if err := f.valid(fact); err != nil {
		return false, err
	}
	return f.merge(fact), nil
}

// merge adds a fact Parse read or Add checked; identical facts merge, retired wins.
func (f *File) merge(fact Fact) bool {
	i, found := slices.BinarySearchFunc(f.facts, fact, compareFacts)
	if !found {
		f.facts = slices.Insert(f.facts, i, fact)
		return true
	}
	if fact.Retired && !f.facts[i].Retired {
		f.facts[i].Retired = true
		return true
	}
	return false
}

// Format is the canonical text of the lock: the header, then one line per fact (LOCK.md §2.3).
func (f *File) Format() []byte {
	out := []byte(header + lineBreak)
	for _, fact := range f.facts {
		out = fact.appendLine(out)
		out = append(out, lineBreak...)
	}
	return out
}

// appendLine writes the fact with the kind padded to its column and two-space separators.
func (fact Fact) appendLine(out []byte) []byte {
	kind := fact.Kind.String()
	out = append(out, kind...)
	out = append(out, padding[:kindWidth-len(kind)]...)
	out = append(out, separator...)
	out = append(out, fact.writtenName()...)
	if fact.Kind != KindTable {
		out = append(out, separator...)
		out = fact.Value.appendText(out)
	}
	out = append(out, separator...)
	out = append(out, fact.Holder...)
	if fact.Retired {
		out = append(out, separator...)
		out = append(out, retiredWord...)
	}
	return out
}

// writtenName is the name as the line writes it: a field fact adds `.<field>`.
func (fact Fact) writtenName() string {
	if fact.Kind == KindField {
		return fact.Name + nameSep + fact.Field
	}
	return fact.Name
}

// appendText writes an integer in canonical decimal, a string JSON-quoted (WIRE.md §7.3).
func (v Value) appendText(out []byte) []byte {
	if v.IsString {
		return diag.AppendJSONString(out, v.Str)
	}
	return strconv.AppendInt(out, v.Int, decimalBase)
}

// compareFacts is the canonical order: kind, name, value, holder (LOCK.md §2.3).
func compareFacts(a, b Fact) int {
	return cmp.Or(
		cmp.Compare(a.Kind.String(), b.Kind.String()),
		cmp.Compare(a.writtenName(), b.writtenName()),
		compareValues(a.Value, b.Value),
		cmp.Compare(a.Holder, b.Holder),
	)
}

// compareValues orders integers by value and strings by bytes, integers first.
func compareValues(a, b Value) int {
	return cmp.Or(cmp.Compare(a.rank(), b.rank()), cmp.Compare(a.Int, b.Int), cmp.Compare(a.Str, b.Str))
}

func (v Value) rank() int {
	if v.IsString {
		return 1
	}
	return 0
}
