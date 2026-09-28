package progen_test

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// Operators on the JSON files a load.dir reads: each edits one member of a data file, or
// declares a field and gives it a bad value there.
func wireOperators() []operator {
	return []operator{
		op(diag.E3203.Def().Code, "WIRE.md §5.1 (Duration with a fraction of a ms)", onMembers(ofType(durationType), editValue(".0000001"))),
		op(diag.E3315.Def().Code, "WIRE.md §5.4 (null for a field not optional)", onMembers(notOptional, setValue("null"))),
		op(diag.E7103.Def().Code, "WIRE.md §5.1 (fraction for an integer)", onMembers(ofType(integerType), editValue(".0"))),
		op(diag.E7104.Def().Code, "WIRE.md §3.2 (duplicate key)", onMembers(anyMember, duplicateMember)),
		op(diag.E7105.Def().Code, "WIRE.md §3.1 (not UTF-8)", onMembers(stringValue, insertInString("\xff"))),
		op(diag.E7105.Def().Code, "WIRE.md §3.2 (unpaired surrogate)", onMembers(stringValue, insertInString(`\uD800`))),
		op(diag.E7109.Def().Code, "WIRE.md §3.1 (trailing comma)", trailingComma),
		op(diag.E7110.Def().Code, "WIRE.md §5.1 (string for a number)", onMembers(numberValue, setValue(`"zz"`))),
		op(diag.E7111.Def().Code, "WIRE.md §5.3 (not a member of the enum)", addField(newField{
			field: `zzKind: ZzKind? @json("zzKind")`, decl: "/// Kind.\nenum ZzKind { small, big }",
			before: `"zzKind": `, focus: `"huge"`,
		})),
		op(diag.E7112.Def().Code, "WIRE.md §5.6 (variant object with an unknown case)", addField(newField{
			field:  `zzVariant: ZzVariant? @json("zzVariant")`,
			decl:   "/// Variant.\nvariant ZzVariant {\n  /// Small.\n  small {\n    /// N.\n    n: Int = 0\n  }\n}",
			before: `"zzVariant": {"kind": `, focus: `"zznope"`, after: `}`,
		})),
		op(diag.E7114.Def().Code, "WIRE.md §5.7 (table key not an identifier)", addField(newField{
			field: `zzSubs: (table ZzSub)? @json("zzSubs")`, decl: "/// Sub.\nrecord ZzSub {\n  /// N.\n  n: Int\n}",
			before: `"zzSubs": {`, focus: `"a b"`, after: `: {"n": 1}}`,
		})),
		op(diag.E7117.Def().Code, "WIRE.md §5.14 (pairs slot with one key but not the other)", addField(newField{
			field: `zzPairs: [ZzPair](..=2) = [] @json(pairs: ["zzK{i}", "zzV{i}"])`,
			decl:  "/// Pair.\nrecord ZzPair {\n  /// K.\n  k: Int\n  /// V.\n  v: Int\n}",
			focus: `"zzK0"`, after: `: 1`,
		})),
	}
}

// member is one member of a data file's top-level object: its key and value, and their bytes.
type member struct {
	raw              string
	keyStart, keyEnd int
	valStart, valEnd int
	field            *syntax.FieldDecl // the field of the loaded record it holds, nil for none
}

// members are the members of tg's top-level object, nil when it is not one.
func members(tg target, l loaded) []member {
	dec := json.NewDecoder(bytes.NewReader(tg.src))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil
	}
	var out []member
	for dec.More() {
		prev := int(dec.InputOffset())
		t, err := dec.Token()
		key, ok := t.(string)
		keyEnd := int(dec.InputOffset())
		var raw json.RawMessage
		if err != nil || !ok || dec.Decode(&raw) != nil {
			return nil
		}
		valEnd := int(dec.InputOffset())
		out = append(out, member{
			raw: string(raw), keyStart: prev + bytes.IndexByte(tg.src[prev:keyEnd], '"'), keyEnd: keyEnd,
			valStart: valEnd - len(raw), valEnd: valEnd, field: l.fields[key],
		})
	}
	return out
}

type memberEdit func(tg target, m member) progen.Site

// onMembers applies f to each member keep accepts of each data file a load.dir reads.
func onMembers(keep func(target, member) bool, f memberEdit) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		l, ok := loadedBy(tg)
		if !ok {
			return nil
		}
		var out []progen.Site
		for _, m := range members(tg, l) {
			if keep(l.src, m) {
				out = append(out, f(tg, m))
			}
		}
		return out
	}
}

var (
	integerType  = regexp.MustCompile(`^U?Int(8|16|32|64)?\b`)
	durationType = regexp.MustCompile(`^Duration\b`)
	integerRaw   = regexp.MustCompile(`^-?[0-9]+$`)
)

// ofType keeps the members of an integer token whose field's type matches re.
func ofType(re *regexp.Regexp) func(target, member) bool {
	return func(src target, m member) bool {
		return m.field != nil && re.MatchString(text(src, m.field.Type)) && integerRaw.MatchString(m.raw)
	}
}

func notOptional(src target, m member) bool {
	return m.field != nil && !strings.HasSuffix(text(src, m.field.Type), "?")
}

func anyMember(target, member) bool { return true }

func stringValue(_ target, m member) bool { return strings.HasPrefix(m.raw, `"`) }

func numberValue(_ target, m member) bool { return strings.ContainsAny(m.raw[:1], "-0123456789") }

// editValue appends suffix to the value; the focus is the whole value.
func editValue(suffix string) memberEdit {
	return func(tg target, m member) progen.Site { return site(replace(m.valStart, m.valEnd, m.raw+suffix)) }
}

func setValue(v string) memberEdit {
	return func(tg target, m member) progen.Site { return site(replace(m.valStart, m.valEnd, v)) }
}

// insertInString puts s first inside the string value; the finding is at s.
func insertInString(s string) memberEdit {
	return func(tg target, m member) progen.Site { return site(insert(m.valStart+1, s)) }
}

// duplicateMember repeats the member right after it; the finding is at the copy's key.
func duplicateMember(tg target, m member) progen.Site {
	key := string(tg.src[m.keyStart:m.keyEnd])
	return seq(2, mark(tg, m.keyStart, m.keyEnd), insert(m.valEnd, ", "), insert(m.valEnd, key), insert(m.valEnd, ": "+m.raw))
}

// trailingComma puts a comma after the last member; the finding is at the closing brace.
func trailingComma(tg target) []progen.Site {
	l, ok := loadedBy(tg)
	if !ok {
		return nil
	}
	ms := members(tg, l)
	if len(ms) == 0 {
		return nil
	}
	last := ms[len(ms)-1]
	end := last.valEnd + bytes.IndexByte(tg.src[last.valEnd:], '}')
	return []progen.Site{seq(1, insert(last.valEnd, ","), mark(tg, end, end+1))}
}

// newField is a field a wire operator declares last in the loaded record, with its doc, the
// declarations it needs, and the member it gets first in the data file: before+focus+after,
// the finding at focus.
type newField struct {
	field, decl          string
	before, focus, after string
}

// addField declares n's field and declarations beside the record a load.dir reads the data
// file tg as, and gives the field n's member in tg.
func addField(n newField) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		l, ok := loadedBy(tg)
		last := lastField(l)
		if !ok || last == nil || !endsLine(l.src, last) {
			return nil
		}
		open := bytes.IndexByte(tg.src, '{') + 1
		data := string(tg.src[:open]) + n.before + n.focus + n.after + "," + string(tg.src[open:])
		_, e := span(l.src, last)
		at := lineEnd(l.src, e) + 1
		ind := indent(l.src, e)
		end := declEnd(l.src)
		return []progen.Site{{
			Path:      l.src.path,
			Edits:     []progen.Edit{insert(at, ind+"/// Zz.\n"+ind+n.field+"\n"), insert(end, "\n\n"+n.decl+"\n")},
			Add:       map[string][]byte{tg.path: []byte(data)},
			Elsewhere: &progen.Place{Path: tg.path, Region: progen.Region{Start: open + len(n.before), End: open + len(n.before) + len(n.focus)}},
		}}
	}
}

// lastField is the last field of l's record, nil for none.
func lastField(l loaded) *syntax.FieldDecl {
	if l.record == nil {
		return nil
	}
	fs := fieldsOf(l.record)
	if len(fs) == 0 {
		return nil
	}
	return fs[len(fs)-1]
}
