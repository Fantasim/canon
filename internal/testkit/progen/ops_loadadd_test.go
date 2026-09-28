package progen_test

import (
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// Operators that add a load call and a small fixture beside a source file, self-contained.
func loadAddOperators() []operator {
	return []operator{
		op(diag.E3317.Def().Code, "WIRE.md §5.8 (two map keys that encode to the same text)", mapKeyCollision),
		op(diag.E7006.Def().Code, "WIRE.md §6.1 (option not listed for the detected format)", loadOptionNotListed),
		op(diag.E7102.Def().Code, "WIRE.md §6.8 (a define redefined with a different value)", defineTwice),
		op(diag.E7106.Def().Code, "WIRE.md §6.3 (an `at:` path missing in its document)", loadAtMissing),
		op(diag.E7108.Def().Code, "WIRE.md §6.6 (a CSV cell not of its field's type)", csvBadCell),
		op(diag.E7113.Def().Code, "WIRE.md §6.6 (an unclosed CSV quote)", csvUnclosedQuote),
		op(diag.W7101.Def().Code, "WIRE.md §6.8 (a function-like macro skipped)", macroSkipped),
	}
}

// mapKeyCollision loads a map whose "-0" key encodes to the same text as its "0" key.
func mapKeyCollision(tg target) []progen.Site {
	if !isSource(tg) || dataMode(tg) {
		return nil
	}
	end := declEnd(tg)
	dataPath := path.Join(path.Dir(tg.path), "zzmap.json")
	decl := `local let zzMap: {Int: Int} = load("zzmap.json")`
	head, focus, tail := `{"0": 1, `, `"-0"`, `: 2}`+"\n"
	return []progen.Site{{
		Path:  tg.path,
		Edits: []progen.Edit{insert(end, "\n\n"+decl+"\n")},
		Add:   map[string][]byte{dataPath: []byte(head + focus + tail)},
		Elsewhere: &progen.Place{
			Path:   dataPath,
			Region: progen.Region{Start: len(head), End: len(head) + len(focus)},
		},
	}}
}

// loadOptionNotListed loads a trivial JSON file with `prefix:`, an option no JSON load lists.
func loadOptionNotListed(tg target) []progen.Site {
	if !isSource(tg) || dataMode(tg) {
		return nil
	}
	end := declEnd(tg)
	dataPath := path.Join(path.Dir(tg.path), "zzopt.json")
	before, focus := `local let zzOpt: {String: Int}? = `, `load("zzopt.json", prefix: "A")`
	s := seq(1, insert(end, "\n\n"+before), insert(end, focus), insert(end, "\n"))
	s.Add = map[string][]byte{dataPath: []byte("{}\n")}
	return []progen.Site{s}
}

// loadAtMissing loads a trivial JSON file with an `at:` path it does not hold.
func loadAtMissing(tg target) []progen.Site {
	if !isSource(tg) || dataMode(tg) {
		return nil
	}
	end := declEnd(tg)
	dataPath := path.Join(path.Dir(tg.path), "zzat.json")
	decl := `local let zzAt: {String: Int}? = load("zzat.json", at: "nope")`
	root := "{}"
	return []progen.Site{{
		Path:  tg.path,
		Edits: []progen.Edit{insert(end, "\n\n"+decl+"\n")},
		Add:   map[string][]byte{dataPath: []byte(root + "\n")},
		Elsewhere: &progen.Place{
			Path:   dataPath,
			Region: progen.Region{Start: 0, End: len(root)},
		},
	}}
}

// defineTwice loads a header whose only #define is given a second, different value.
func defineTwice(tg target) []progen.Site {
	if !isSource(tg) || dataMode(tg) {
		return nil
	}
	end := declEnd(tg)
	dataPath := path.Join(path.Dir(tg.path), "zzdefines.h")
	data := "#define ZZ_A 1\n#define ZZ_A 2\n"
	decl := `local let zzDefines = load.defines("zzdefines.h", prefix: "ZZ_")`
	hash := strings.Index(data, "\n") + 1 // the second line's '#'
	return []progen.Site{{
		Path:  tg.path,
		Edits: []progen.Edit{insert(end, "\n\n"+decl+"\n")},
		Add:   map[string][]byte{dataPath: []byte(data)},
		Elsewhere: &progen.Place{
			Path:   dataPath,
			Region: progen.Region{Start: hash, End: hash + 1},
		},
	}}
}

// macroSkipped loads a header whose second define is a function-like macro, skipped.
func macroSkipped(tg target) []progen.Site {
	if !isSource(tg) || dataMode(tg) {
		return nil
	}
	end := declEnd(tg)
	dataPath := path.Join(path.Dir(tg.path), "zzmacro.h")
	data := "#define ZZ_A 1\n#define ZZ_F(x) 2\n"
	decl := `local let zzMacro = load.defines("zzmacro.h", prefix: "ZZ_")`
	hash := strings.Index(data, "\n") + 1 // the second line's '#'
	return []progen.Site{{
		Path:  tg.path,
		Edits: []progen.Edit{insert(end, "\n\n"+decl+"\n")},
		Add:   map[string][]byte{dataPath: []byte(data)},
		Elsewhere: &progen.Place{
			Path:   dataPath,
			Region: progen.Region{Start: hash, End: hash + 1},
		},
	}}
}

// csvBadCell loads a one-column CSV whose only data cell does not parse as its field's Int.
func csvBadCell(tg target) []progen.Site {
	if !isSource(tg) || dataMode(tg) {
		return nil
	}
	end := declEnd(tg)
	dataPath := path.Join(path.Dir(tg.path), "zzcell.csv")
	data := "a\nnope\n"
	decl := "local record ZzCsvCell {\n  a: Int\n}\n\n" +
		`local let zzCsvCell: [ZzCsvCell] = load.csv("zzcell.csv", header: true)`
	cell := strings.Index(data, "nope")
	return []progen.Site{{
		Path:  tg.path,
		Edits: []progen.Edit{insert(end, "\n\n"+decl+"\n")},
		Add:   map[string][]byte{dataPath: []byte(data)},
		Elsewhere: &progen.Place{
			Path:   dataPath,
			Region: progen.Region{Start: cell, End: cell + len("nope")},
		},
	}}
}

// csvUnclosedQuote loads a one-column CSV whose only data cell opens a quote it never closes.
func csvUnclosedQuote(tg target) []progen.Site {
	if !isSource(tg) || dataMode(tg) {
		return nil
	}
	end := declEnd(tg)
	dataPath := path.Join(path.Dir(tg.path), "zzquote.csv")
	data := "a\n\"x\n"
	decl := "local record ZzCsvQuote {\n  a: String\n}\n\n" +
		`local let zzCsvQuote: [ZzCsvQuote] = load.csv("zzquote.csv", header: true)`
	quote := strings.Index(data, `"`)
	return []progen.Site{{
		Path:  tg.path,
		Edits: []progen.Edit{insert(end, "\n\n"+decl+"\n")},
		Add:   map[string][]byte{dataPath: []byte(data)},
		Elsewhere: &progen.Place{
			Path:   dataPath,
			Region: progen.Region{Start: quote, End: quote + 1},
		},
	}}
}
