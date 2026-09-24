package progen_test

import (
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// Operators on load.dir patterns: each rewrites one call's pattern; the finding is at the call.
func loadOperators() []operator {
	return []operator{
		op(diag.E7004.Def().Code, "WIRE.md §6.1 (glob base directory missing)", onLoadDir(missingBase)),
		op(diag.E7005.Def().Code, "WIRE.md §6.5 (unclosed [ in a glob)", onLoadDir(unclosedClass)),
		{code: diag.E7007.Def().Code, rule: "WIRE.md §6.2 (a matched file of no known format)", many: true, sites: onLoadDir(unknownFormat)},
		op(diag.W7107.Def().Code, "WIRE.md §6.5 (glob matching no file)", onLoadDir(matchesNothing)),
		op(diag.W7115.Def().Code, "WIRE.md §6.5 (symbolic link outside the roots)", linkOutside),
	}
}

// patternEdit is a new pattern for a load.dir of tg, and the files to add beside it.
type patternEdit func(tg target, pattern string) (string, map[string][]byte)

// dirLoad is one load.dir call whose only argument is a plain string: the call and the pattern.
type dirLoad struct {
	call    *syntax.LoadExpr
	arg     syntax.Node
	pattern string
}

// dirLoads are the load.dir calls of tg that read one plain pattern.
func dirLoads(tg target) []dirLoad {
	var out []dirLoad
	for _, e := range nodes[*syntax.LoadExpr](tg) {
		if e.Method == nil || e.Method.Name != "dir" || len(e.Args) != 1 || e.Args[0].Name != nil {
			continue
		}
		s, ok := e.Args[0].Value.(*syntax.StringLit)
		if !ok || s.Multiline || len(s.Parts) != 1 || s.Parts[0].Interp != nil {
			continue
		}
		out = append(out, dirLoad{call: e, arg: s, pattern: s.Parts[0].Text})
	}
	return out
}

// onLoadDir rewrites the pattern of each load.dir of a source file; the focus is the whole call.
func onLoadDir(f patternEdit) func(target) []progen.Site {
	return func(tg target) []progen.Site {
		if !isSource(tg) {
			return nil
		}
		var out []progen.Site
		for _, l := range dirLoads(tg) {
			s, e := span(tg, l.call)
			as, ae := span(tg, l.arg)
			pattern, add := f(tg, l.pattern)
			call := string(tg.src[s:as]) + strconv.Quote(pattern) + string(tg.src[ae:e])
			out = append(out, progen.Site{Edits: []progen.Edit{replace(s, e, call)}, Add: add})
		}
		return out
	}
}

func missingBase(_ target, pattern string) (string, map[string][]byte) {
	return path.Join("zzgone", pattern), nil
}

func unclosedClass(_ target, pattern string) (string, map[string][]byte) {
	return path.Join(path.Dir(pattern), "["+path.Base(pattern)), nil
}

func matchesNothing(_ target, pattern string) (string, map[string][]byte) {
	return pattern + "zz", nil
}

// unknownFormat widens the pattern to every file of its directory and adds one there whose
// extension names no format.
func unknownFormat(tg target, pattern string) (string, map[string][]byte) {
	dir := path.Dir(pattern)
	name := path.Join(path.Dir(tg.path), dir, "zz.xml")
	return path.Join(dir, "*"), map[string][]byte{name: []byte("<zz/>\n")}
}

// linkOutside adds, where a load.dir pattern matches, a symbolic link to a file outside every
// root; the finding is at the call.
func linkOutside(tg target) []progen.Site {
	if !isSource(tg) {
		return nil
	}
	var out []progen.Site
	for _, l := range dirLoads(tg) {
		base := path.Base(l.pattern)
		if !strings.Contains(base, "*") {
			continue
		}
		name := path.Join(path.Dir(tg.path), path.Dir(l.pattern), strings.Replace(base, "*", "zzlink", 1))
		s, e := span(tg, l.call)
		out = append(out, progen.Site{Edits: []progen.Edit{mark(tg, s, e)}, Links: map[string]string{name: "/zzoutside/zz.json"}})
	}
	return out
}

// loaded is the record a load.dir of a data file's package decodes the file as: the source
// file declaring it, the record, and its fields by wire key.
type loaded struct {
	src    target
	record *syntax.RecordDecl
	fields map[string]*syntax.FieldDecl
}

// elemOf reads the element record of a load.dir's declared type: [T], [T] keyed by k, table T.
var elemOf = regexp.MustCompile(`^(?:stable\s+)?(?:table\s+(\w+)|\[(\w+)\])`)

// loadedBy is the record of the load.dir reading the data file tg, if any.
func loadedBy(tg target) (loaded, bool) {
	if tg.file != nil || path.Ext(tg.path) != ".json" {
		return loaded{}, false
	}
	for _, src := range peers(tg) {
		for _, d := range nodes[*syntax.LetDecl](src) {
			if name := readsAs(src, d, tg.path); name != "" {
				return recordOf(tg, name)
			}
		}
	}
	return loaded{}, false
}

// readsAs is the element record name of d when d is a load.dir whose pattern matches name.
func readsAs(src target, d *syntax.LetDecl, name string) string {
	if !isSource(src) || d.Type == nil {
		return ""
	}
	for _, l := range dirLoads(src) {
		if l.call != d.Value {
			continue
		}
		ok, err := path.Match(path.Join(path.Dir(src.path), l.pattern), name)
		m := elemOf.FindStringSubmatch(text(src, d.Type))
		if ok && err == nil && m != nil {
			return m[1] + m[2]
		}
	}
	return ""
}

// recordOf is the record named name in tg's package, its fields by wire key.
func recordOf(tg target, name string) (loaded, bool) {
	for _, src := range peers(tg) {
		for _, r := range nodes[*syntax.RecordDecl](src) {
			if r.Name == nil || r.Name.Name != name || r.Body == nil {
				continue
			}
			return loaded{src: src, record: r, fields: byWireKey(src, r)}, true
		}
	}
	return loaded{}, false
}

// byWireKey are the fields of r by wire key.
func byWireKey(src target, r *syntax.RecordDecl) map[string]*syntax.FieldDecl {
	out := map[string]*syntax.FieldDecl{}
	for _, f := range fieldsOf(r) {
		out[wireKey(src, f)] = f
	}
	return out
}

// wireKey is a field's JSON key: the positional string of its @json, else its name (WIRE.md §5.5.2).
func wireKey(src target, f *syntax.FieldDecl) string {
	for _, a := range f.Annotations {
		if a.Name == nil || a.Name.Name != "json" {
			continue
		}
		for _, arg := range a.Args {
			if arg.Name != nil {
				continue
			}
			if k, err := strconv.Unquote(text(src, arg.Value)); err == nil {
				return k
			}
		}
	}
	return f.Name.Name
}
