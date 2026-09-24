package gogen

import (
	"fmt"
	"strconv"
	"strings"
)

// location is a place in a data file as generated code spells it: a format of %s (path, key
// locals) and %d (indexes) verbs, and their args.
type location struct {
	format string
	args   []string
}

// keyLoc is the location of a literal key under the current path.
func (g *gen) keyLoc(key string) location {
	return location{format: verbString + escapeVerbs(key), args: []string{g.lc.Path}}
}

// root is the current path itself.
func (g *gen) root() location { return location{format: verbString, args: []string{g.lc.Path}} }

// dot appends a dot: the prefix of what the location holds.
func (l location) dot() location { return location{l.format + dot, l.args} }

// arg appends the string held by the local v.
func (l location) arg(v string) location {
	return location{l.format + verbString, append(append([]string(nil), l.args...), v)}
}

// index appends `[i]`, i a local int.
func (l location) index(i string) location {
	return location{l.format + indexVerb, append(append([]string(nil), l.args...), i)}
}

// locExpr is the location as a Go string: path+"key", else a fmt.Sprintf.
func (g *gen) locExpr(l location) string {
	rest, ok := strings.CutPrefix(l.format, verbString)
	if ok && len(l.args) == 1 && !strings.Contains(strings.ReplaceAll(rest, percentPercent, ""), percent) {
		if rest == "" {
			return l.args[0]
		}
		return l.args[0] + plus + strconv.Quote(strings.ReplaceAll(rest, percentPercent, percent))
	}
	return fmt.Sprintf(sprintfFormat, g.use(fmtPkg, fmtPkg), strconv.Quote(l.format), strings.Join(l.args, listSep))
}

// errAt is the error of a failed read at l, naming value unless "".
func (g *gen) errAt(l location, what, value string) string {
	args := append([]string{g.lc.Name}, l.args...)
	if value != "" {
		args = append(args, value)
	}
	format := strconv.Quote(verbString + keyValueSep + l.format + keyValueSep + what)
	return fmt.Sprintf(errorfFormat, g.use(fmtPkg, fmtPkg), format, strings.Join(args, listSep))
}

// wrapAt is jsonError of err at l: the key path and the JSON kind found, never Go's types.
func (g *gen) wrapAt(l location) string {
	return fmt.Sprintf(wrapFormat, helperError, g.lc.Name, g.locExpr(l), g.lc.Err)
}

func escapeVerbs(s string) string { return strings.ReplaceAll(s, percent, percentPercent) }
