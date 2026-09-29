package edit

import (
	"path"
	"strconv"
	"strings"
	"unicode"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// newEntryFile adds rec, with key, to a collection whose entries live in files (API.md N1): a
// new file placed by the let's @files template (N2, N3) or by N4, holding what N5 says.
func (x *opCtx) newEntryFile(rec *value.Record, key value.Value) error {
	let, ok := x.res.root.obj.Decl().(*syntax.LetDecl)
	if !ok || len(x.res.Steps) > 0 {
		return &NotEditableError{Reason: ReasonOrder}
	}
	load, isLoad := syntax.Unparen(let.Value).(*syntax.LoadExpr)
	display, err := x.entryPath(let, load, rec, key)
	if err != nil {
		return err
	}
	if err := x.a.free(display); err != nil {
		return err
	}
	var content []byte
	if isLoad {
		content, err = x.jsonEntryFile(rec)
	} else {
		content, err = x.canonEntryFile(let, rec, key)
	}
	if err != nil {
		return err
	}
	x.w.creates = append(x.w.creates, newFile{display: display, content: content})
	x.w.own(display, x.res.root.pkg.Path)
	return nil
}

// entryPath is the display path of a new entry's file: the @files template expanded in the
// package directory (N2, N3), else <package dir>/<value name>/<key>.canon or the load.dir
// glob's directory (N4).
func (x *opCtx) entryPath(let *syntax.LetDecl, load *syntax.LoadExpr, rec *value.Record, key value.Value) (string, error) {
	dir := packageDir(x.res.root.pkg)
	keyText, err := templateText(key)
	if err != nil {
		return "", err
	}
	if tpl, ok := filesTemplate(let); ok {
		rel, err := x.expand(tpl, rec, keyText)
		if err != nil {
			return "", err
		}
		return path.Join(dir, rel), nil
	}
	if load == nil {
		return path.Join(dir, x.res.root.obj.Name(), keyText+project.SourceExt), nil
	}
	return globPlace(x.res.root.obj.File(), load, keyText)
}

// packageDir is the display path of a package's directory: its name, '/'-separated.
func packageDir(pkg *check.Package) string {
	return strings.ReplaceAll(pkg.Path, string(fieldMark), pathSep)
}

// filesTemplate is the let's @files template.
func filesTemplate(d *syntax.LetDecl) (string, bool) {
	for _, a := range d.Annotations {
		if a.Name == nil || a.Name.Name != syntax.AnnFiles || len(a.Args) == 0 {
			continue
		}
		if s, ok := a.Args[0].Value.(*syntax.StringLit); ok {
			return templateOf(s)
		}
	}
	return "", false
}

// templateOf is a template string as written: its text, each `{f.g}` a name or a field path
// (GRAMMAR.md @files); false for any other interpolation.
func templateOf(s *syntax.StringLit) (string, bool) {
	var b strings.Builder
	for _, p := range s.Parts {
		if p.Interp == nil {
			b.WriteString(p.Text)
			continue
		}
		name, ok := namePath(p.Interp.X)
		if !ok || p.Interp.Spec != nil {
			return "", false
		}
		b.WriteString(string(tplOpen) + name + string(tplClose))
	}
	return b.String(), true
}

// namePath is `a.b.c` written as names and selectors.
func namePath(e syntax.Expr) (string, bool) {
	switch x := e.(type) {
	case *syntax.IdentExpr:
		return x.Name, true
	case *syntax.SelectorExpr:
		head, ok := namePath(x.X)
		return head + string(fieldMark) + x.Name.Name, ok
	}
	return "", false
}

// expand fills a template's `{id}`, `{f}` and `{f.g}` from the new entry (API.md N2).
func (x *opCtx) expand(tpl string, rec *value.Record, key string) (string, error) {
	var b strings.Builder
	for {
		open := strings.IndexByte(tpl, tplOpen)
		if open < 0 {
			b.WriteString(tpl)
			return b.String(), nil
		}
		end := strings.IndexByte(tpl[open:], tplClose)
		if end < 0 {
			return "", errTemplate
		}
		name := tpl[open+1 : open+end]
		text := key
		if name != pseudoID {
			v, err := x.templateField(rec, strings.Split(name, string(fieldMark)))
			if err != nil {
				return "", err
			}
			if text, err = templateText(v); err != nil {
				return "", err
			}
		}
		b.WriteString(tpl[:open] + text)
		tpl = tpl[open+end+1:]
	}
}

// templateField is the value at a template's field path through records and the current case
// of variants, a field left out taking its default.
func (x *opCtx) templateField(rec *value.Record, names []string) (value.Value, error) {
	var cur value.Value = rec
	for _, name := range names {
		r, ok := cur.(*value.Record)
		if !ok {
			return nil, &ValueError{Expected: pathSegment, Got: name, Detail: errTemplate.Error()}
		}
		i := fieldIndex(fieldsOf(r.T), name)
		if i < 0 {
			return nil, errTemplate
		}
		cur = r.Fields[i]
		if cur == nil {
			cur, _ = x.a.defaultOf(r, i)
		}
	}
	return cur, nil
}

// templateText is a value written in a path (API.md N2): a member's or a case's wire name, a
// ref's key, an integer, a Bool, a String; none, a text a path cannot hold, and one starting with
// "." (a hidden file the scan skips, log-2026-09-29 U4c final) are ErrBadValue.
func templateText(v value.Value) (string, error) {
	var text string
	switch x := v.(type) {
	case *value.Member:
		text = x.Enum.Members[x.Index].Wire
	case *value.Record:
		c, ok := x.T.Base().(*types.CaseType)
		if !ok {
			return "", badTemplate(v)
		}
		text = c.Wire
	case *value.Ref:
		text = x.Key.Text()
	case *value.Int:
		text = strconv.FormatInt(x.V, decimalBase)
	case *value.Bool, *value.Str:
		text = x.CanonText()
	default:
		return "", badTemplate(v)
	}
	if text == "" || strings.HasPrefix(text, dotSeg) || strings.ContainsFunc(text, badPathRune) {
		return "", badTemplate(v)
	}
	return text, nil
}

func badPathRune(r rune) bool {
	return r == '/' || r == '\\' || unicode.IsControl(r)
}

// badTemplate is the ValueError of a value no path can hold (API.md N2).
func badTemplate(v value.Value) error {
	got := noneWord
	if v != nil {
		got = v.CanonText()
	}
	return &ValueError{Expected: pathSegment, Got: got, Detail: errTemplate.Error()}
}

// globPlace is where a new load.dir element goes without @files (API.md N4): the glob's
// directory, when only its last segment has a wildcard, as <key> and the glob's extension,
// which the last segment must match; else order.
func globPlace(f *syntax.File, load *syntax.LoadExpr, key string) (string, error) {
	pattern, ok := loadPattern(load)
	if !ok {
		return "", &NotEditableError{Reason: ReasonOrder}
	}
	dir, last := path.Split(pattern)
	name := key + path.Ext(last)
	if strings.ContainsAny(dir, globMagic) || strings.ContainsAny(last, globBraces) {
		return "", &NotEditableError{Reason: ReasonOrder}
	}
	if match, err := path.Match(last, name); err != nil || !match {
		return "", &NotEditableError{Reason: ReasonOrder}
	}
	if strings.HasPrefix(dir, string(rootMark)) {
		return path.Join(dir, name), nil
	}
	return path.Join(path.Dir(f.Src.Path), dir, name), nil
}

// loadPattern is the literal pattern of a load.dir.
func loadPattern(x *syntax.LoadExpr) (string, bool) {
	if len(x.Args) == 0 || x.Args[0].Name != nil {
		return "", false
	}
	s, ok := x.Args[0].Value.(*syntax.StringLit)
	if !ok {
		return "", false
	}
	return constString(s)
}

// canonEntryFile is a new entry file (API.md N5): the package line, a blank line, and
// `entry <value>.<key> { … }` printed canonically; a keyed list's body leaves its key out.
func (x *opCtx) canonEntryFile(let *syntax.LetDecl, rec *value.Record, key value.Value) ([]byte, error) {
	keyText, err := entryKeyText(key)
	if err != nil {
		return nil, err
	}
	body := rec
	if lt, ok := baseOf(present(x.res.root.obj.Type())).(*types.ListType); ok && lt.KeyedBy != nil {
		body = withField(rec, lt.KeyedBy.Index, nil)
	}
	text, err := x.a.entryText(let.Name.Name+string(fieldMark)+keyText, body, false)
	if err != nil {
		return nil, err
	}
	src := packageWord + space + x.res.root.pkg.Path + blankLine + entryWord + space + text + newline
	return freshFile(x.res.root.pkg.Path, src)
}

// entryKeyText is a key as an entry declaration writes it: a word or an integer.
func entryKeyText(key value.Value) (string, error) {
	text, err := templateText(key)
	if _, perr := strconv.ParseUint(text, decimalBase, int64Bits); err != nil || !isWord(text) && perr != nil {
		return "", badTemplate(key)
	}
	return text, nil
}

// freshFile is src parsed, its kind read from its header, and printed as a new file (FORMATTER.md §13 step 7).
func freshFile(pkg, src string) ([]byte, error) {
	var fs source.FileSet
	f, err := fs.Add(pkg+project.SourceExt, pkg+project.SourceExt, []byte(src))
	if err != nil {
		return nil, err
	}
	bag := diag.NewBag(&fs, pkg)
	tree := syntax.Parse(f, syntax.FileSource, bag)
	if bag.Summary().Errors > 0 {
		return nil, errUnprinted
	}
	return format.Fresh(tree)
}

// jsonEntryFile is a new JSON element file: its wire form in canonical source layout (N5).
func (x *opCtx) jsonEntryFile(rec *value.Record) ([]byte, error) {
	n, err := x.a.entryNode(rec)
	if err != nil {
		return nil, err
	}
	return jsonsrc.Format(n), nil
}
