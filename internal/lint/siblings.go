package lint

import (
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
)

// sibling is a field of a numbered family: its declaration, its position among the holder's fields and its number.
type sibling struct {
	df    declaredField
	index int
	num   int
}

// family is the numbered siblings of one stem and one written type, in source order.
type family struct {
	stem, typ string
	members   []sibling
}

// numberedSiblings reports W0002 (API.md L3): a family no load and no output reaches, and no unloaded importer can load, is
// one list; two families on the wire that share the numbers 0 to n-1 and that a pairs list maps are two columns of one;
// any other shape is not reported.
func numberedSiblings(p *pass, pkg *check.Package, bag *diag.Bag) {
	p.eachHolder(pkg, func(h holder) {
		fams := familiesOf(h)
		switch {
		case len(fams) == 0:
		case !p.wire.Has(h.typ) && p.importersLoaded(h.pkg):
			for _, fam := range fams {
				first, last := fam.ends()
				diag.W0002.AtList(h.file.Span(first.df.decl.Name), first.df.field.Name, last.df.field.Name).Report(bag)
			}
		case len(fams) == pairFamilies && sharesSlots(fams[0], fams[1]):
			if keys, ok := wireStems(fams[0], fams[1]); ok {
				first, last := extremes(fams...)
				diag.W0002.AtPairs(h.file.Span(first.df.decl.Name), first.df.field.Name, last.df.field.Name, keys[0], keys[1]).Report(bag)
			}
		}
	})
}

// wireStems are the stems of the wire names of two families, the keys of their pairs list, when WIRE.md 5.14 can map them:
// each family's wire names are one stem and its member's number, the stems differ, and the elements have a scalar wire form.
func wireStems(a, b family) ([pairFamilies]string, bool) {
	var stems [pairFamilies]string
	for i, f := range []family{a, b} {
		stem, ok := f.wireStem()
		if !ok || !f.scalar() {
			return stems, false
		}
		stems[i] = stem
	}
	return stems, stems[0] != stems[1]
}

// wireStem is the stem all of f's wire names share before their number, false when a wire name is a path or has another ending.
func (f family) wireStem() (string, bool) {
	stem := ""
	for i, m := range f.members {
		fld := m.df.field
		cut, ok := strings.CutSuffix(fld.Wire, strconv.Itoa(m.num))
		if len(fld.WirePath) > 1 || !ok || cut == "" || unicode.IsDigit(rune(cut[len(cut)-1])) || i > 0 && cut != stem {
			return "", false
		}
		stem = cut
	}
	return stem, true
}

// scalar reports a family whose members all have a scalar wire form: a Bool, number, String, Duration, enum, ref or literal union, not optional.
func (f family) scalar() bool {
	return !slices.ContainsFunc(f.members, func(m sibling) bool { return !slices.Contains(scalarKinds, m.df.field.Type.Kind()) })
}

// familiesOf are the families of h that hold two or more fields with distinct numbers, none with `@json(pairs:)`, by first appearance.
func familiesOf(h holder) []family {
	var all []family
	for i, df := range h.declaredFields() {
		if df.field.Pairs != nil || df.decl.Type == nil {
			continue
		}
		stem, num, ok := splitNumber(df.field.Name)
		if !ok {
			continue
		}
		typ := writtenType(h, df)
		at := slices.IndexFunc(all, func(f family) bool { return f.stem == stem && f.typ == typ })
		if at < 0 {
			all = append(all, family{stem: stem, typ: typ})
			at = len(all) - 1
		}
		all[at].members = append(all[at].members, sibling{df: df, index: i, num: num})
	}
	return slices.DeleteFunc(all, func(f family) bool { return len(f.members) < minSiblings || !f.distinct() })
}

// extremes are the first and the last in source order of the members of fams, which hold one at least.
func extremes(fams ...family) (first, last sibling) {
	first, last = fams[0].members[0], fams[0].members[0]
	for _, f := range fams {
		for _, m := range f.members {
			if m.index < first.index {
				first = m
			}
			if m.index > last.index {
				last = m
			}
		}
	}
	return first, last
}

// writtenType is the type of df as its declaration writes it, spaces collapsed.
func writtenType(h holder, df declaredField) string {
	span := h.file.Span(df.decl.Type)
	return strings.Join(strings.Fields(string(h.file.Src.Content[span.Start:span.End])), typeSeparator)
}

// splitNumber is name as a stem and the decimal number that ends it; false for a name without a stem or a number.
func splitNumber(name string) (stem string, num int, ok bool) {
	cut := len(name)
	for cut > 0 && unicode.IsDigit(rune(name[cut-1])) {
		cut--
	}
	digits := name[cut:]
	if cut == 0 || digits == "" || len(digits) > maxNumberDigits {
		return "", 0, false
	}
	num, err := strconv.Atoi(digits)
	return name[:cut], num, err == nil
}

// distinct reports a family whose members have no number twice.
func (f family) distinct() bool {
	seen := map[int]bool{}
	for _, m := range f.members {
		if seen[m.num] {
			return false
		}
		seen[m.num] = true
	}
	return true
}

// ends are the members of f with the lowest and the highest number.
func (f family) ends() (first, last sibling) {
	first, last = f.members[0], f.members[0]
	for _, m := range f.members {
		if m.num < first.num {
			first = m
		}
		if m.num > last.num {
			last = m
		}
	}
	return first, last
}

// sharesSlots reports two families whose numbers are both 0 to n-1 (WIRE.md §5.14: the slots of one pairs list).
func sharesSlots(a, b family) bool {
	return len(a.members) == len(b.members) && a.slots() && b.slots()
}

// slots reports a family whose numbers are exactly 0 to n-1.
func (f family) slots() bool {
	nums := make([]int, len(f.members))
	for i, m := range f.members {
		nums[i] = m.num
	}
	slices.Sort(nums)
	for i, n := range nums {
		if n != firstSlot+i {
			return false
		}
	}
	return true
}
