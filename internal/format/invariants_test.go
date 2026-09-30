package format_test

import (
	"bytes"
	"cmp"
	"fmt"
	"math/big"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// positional are the types the tree comparison skips: positions, spans and the token stream.
var positional = map[reflect.Type]bool{
	reflect.TypeFor[syntax.Tok]():     true,
	reflect.TypeFor[syntax.Bounds]():  true,
	reflect.TypeFor[syntax.Delims]():  true,
	reflect.TypeFor[source.Pos]():     true,
	reflect.TypeFor[[]syntax.Token](): true,
	reflect.TypeFor[*source.File]():   true,
}

var bigIntType = reflect.TypeFor[*big.Int]()

// checkFormatted checks the guarantees of FORMATTER.md §1 and §12 on out, the layout of in.
func checkFormatted(t *testing.T, path string, in parsed, out []byte) {
	t.Helper()
	re := parse(t, path, out)
	if got, want := codes(re.bag), codes(in.bag); !slices.Equal(got, want) {
		t.Fatalf("%s: findings %v after formatting, %v before:\n%s", path, got, want, out)
	}
	normalize(in.file)
	normalize(re.file)
	if err := sameTree(reflect.ValueOf(in.file), reflect.ValueOf(re.file), "File"); err != nil {
		t.Fatalf("%s: the tree changed: %v\n%s", path, err, out)
	}
	got, gotImports := comments(re.file)
	want, wantImports := comments(in.file)
	if !slices.Equal(got, want) || !slices.Equal(gotImports, wantImports) {
		t.Fatalf("%s: comments changed:\n%q\n%q\n%s", path, got, want, out)
	}
	if again := mustFile(t, re.file); !bytes.Equal(again, out) {
		t.Fatalf("%s: not idempotent:\n%s", path, lineDiff(out, again))
	}
}

// mustFile prints a tree that parsed without error.
func mustFile(t *testing.T, f *syntax.File) []byte {
	t.Helper()
	out, err := format.File(f)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// codes are the codes of a parse's findings, the byte order mark's aside (§10 removes it).
func codes(bag *diag.Bag) []diag.Code {
	var out []diag.Code
	for _, f := range bag.Findings() {
		if f.Code != diag.E1123.Def().Code {
			out = append(out, f.Code)
		}
	}
	slices.Sort(out)
	return out
}

// normalize sorts the imports and imported names, the one reordering §9.1 makes.
func normalize(f *syntax.File) {
	slices.SortStableFunc(f.Imports, func(x, y *syntax.Import) int {
		return cmp.Or(strings.Compare(dotted(x.Path), dotted(y.Path)), strings.Compare(alias(x), alias(y)))
	})
	for _, imp := range f.Imports {
		slices.SortStableFunc(imp.Names, func(x, y *syntax.Ident) int { return strings.Compare(x.Name, y.Name) })
	}
}

func dotted(q *syntax.QualifiedName) string {
	var parts []string
	for _, p := range q.Parts {
		parts = append(parts, p.Name)
	}
	return strings.Join(parts, ".")
}

func alias(imp *syntax.Import) string {
	if imp.Alias == nil {
		return ""
	}
	return imp.Alias.Name
}

// sameTree compares two trees field by field, positions aside.
func sameTree(a, b reflect.Value, at string) error {
	if a.Type() != b.Type() {
		return fmt.Errorf("%s: %s against %s", at, a.Type(), b.Type())
	}
	if positional[a.Type()] {
		return nil
	}
	switch a.Kind() {
	case reflect.Pointer, reflect.Interface:
		return samePointer(a, b, at)
	case reflect.Struct:
		for i := range a.NumField() {
			name := at + "." + a.Type().Field(i).Name
			if err := sameTree(a.Field(i), b.Field(i), name); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if a.Len() != b.Len() {
			return fmt.Errorf("%s: %d elements against %d", at, a.Len(), b.Len())
		}
		for i := range a.Len() {
			if err := sameTree(a.Index(i), b.Index(i), fmt.Sprintf("%s[%d]", at, i)); err != nil {
				return err
			}
		}
	default:
		if !reflect.DeepEqual(a.Interface(), b.Interface()) {
			return fmt.Errorf("%s: %v against %v", at, a.Interface(), b.Interface())
		}
	}
	return nil
}

func samePointer(a, b reflect.Value, at string) error {
	if a.IsNil() || b.IsNil() {
		if a.IsNil() != b.IsNil() {
			return fmt.Errorf("%s: nil against non-nil", at)
		}
		return nil
	}
	if a.Type() == bigIntType {
		if x, y := a.Interface().(*big.Int), b.Interface().(*big.Int); x.Cmp(y) != 0 {
			return fmt.Errorf("%s: %v against %v", at, x, y)
		}
		return nil
	}
	return sameTree(a.Elem(), b.Elem(), at)
}

// comments are a file's comments with the token after it (commas, colons, separators aside): in
// file order with that token's position, and the import block's sorted with the token they are
// attached to, which moves when the imports and their names are sorted.
func comments(f *syntax.File) (ordered, imports []string) {
	lo, hi := importRange(f)
	kept := format.KeptCommas(f)
	for i, tok := range f.Tokens {
		for _, tr := range tok.Leading {
			ordered, imports = sort2(place(f, tr, i, leadOwner(f, i, kept[i])), lo, hi, ordered, imports)
		}
		for _, tr := range tok.Trailing {
			ordered, imports = sort2(place(f, tr, i+1, trailOwner(f, i, kept[i])), lo, hi, ordered, imports)
		}
	}
	slices.SortStableFunc(imports, func(x, y string) int { return strings.Compare(ownerOf(x), ownerOf(y)) })
	return ordered, imports
}

// ownerSep ends the owner part of an import-block comment's text, which no name holds.
const ownerSep = " | "

// ownerOf is the owner part of an import-block comment: its comments keep their source order.
func ownerOf(imp string) string {
	owner, _, _ := strings.Cut(imp, ownerSep)
	return owner
}

// leadOwner is the owner of an own-line comment before token i (FORMATTER.md §8.1).
func leadOwner(f *syntax.File, i int, keptComma bool) int {
	if keptComma { // DECISIONS 216: the name before it
		return lastAnchored(f, i-1)
	}
	at, _ := anchor(f, i)
	return at
}

// trailOwner is the owner of a comment trailing token i (FORMATTER.md §8.1, DECISIONS 168, 216).
func trailOwner(f *syntax.File, i int, keptComma bool) int {
	t := f.Tokens[i]
	if anchored(t.Kind) {
		return i
	}
	before := lastAnchored(f, i-1)
	if t.Kind != syntax.TokComma || keptComma || !bytes.Contains(f.Src.Content[f.Tokens[before].End:t.Start], []byte("\n")) {
		return before
	}
	at, _ := anchor(f, i+1)
	return at
}

// placed is a comment with the kind of the token after it (a duration's text may change) and
// that token's ordinal; in the import block, with its owner's import path and text.
type placed struct {
	text, imp string
	owner, n  int
	absent    bool
}

// sort2 files a placed comment as an import comment or an ordered one, by its owner.
func sort2(c placed, lo, hi int, ordered, imports []string) ([]string, []string) {
	switch {
	case c.absent:
		return ordered, imports
	case c.owner >= lo && c.owner <= hi:
		return ordered, append(imports, c.imp)
	default:
		return append(ordered, fmt.Sprintf("%d %s", c.n, c.text)), imports
	}
}

// place places a comment of the token before from, or of the token from, attached to owner.
func place(f *syntax.File, tr syntax.Trivia, from, owner int) placed {
	s, ok := commentOf(f, tr)
	if !ok {
		return placed{absent: true}
	}
	at, n := anchor(f, from)
	own := ownLine(f, tr, from)
	tk := f.Tokens[owner]
	return placed{
		text:  fmt.Sprintf("%s %s %v", f.Tokens[at].Kind, s, own),
		imp:   fmt.Sprintf("%s %s%s%s %v", importOf(f, owner), f.Src.Content[tk.Start:tk.End], ownerSep, s, own),
		owner: owner, n: n,
	}
}

// importOf is the path and alias of the import holding token t, or "" outside the imports.
func importOf(f *syntax.File, t int) string {
	for _, imp := range f.Imports {
		if t >= int(imp.First()) && t <= int(imp.Last()) {
			return dotted(imp.Path) + " " + alias(imp)
		}
	}
	return ""
}

// lastAnchored is the last token at or before i that the layout never drops nor adds.
func lastAnchored(f *syntax.File, i int) int {
	for ; i > 0 && !anchored(f.Tokens[i].Kind); i-- {
	}
	return i
}

// ownLine reports a one-line comment that starts its line: a line break separates it from the
// last token before it that the layout keeps; a comment spanning lines is not judged.
func ownLine(f *syntax.File, tr syntax.Trivia, from int) string {
	src := f.Src.Content
	if bytes.Contains(src[tr.Start:tr.End], []byte("\n")) {
		return "*"
	}
	i := from - 1
	for ; i > 0 && !anchored(f.Tokens[i].Kind); i-- {
	}
	if i == 0 {
		return "start"
	}
	return fmt.Sprint(bytes.Contains(src[f.Tokens[i].End:tr.Start], []byte("\n")))
}

// anchor is the first token from i on that the layout never drops nor adds, with its ordinal.
func anchor(f *syntax.File, i int) (at, n int) {
	for ; i < len(f.Tokens)-1 && !anchored(f.Tokens[i].Kind); i++ {
	}
	for _, t := range f.Tokens[:i] {
		if anchored(t.Kind) {
			n++
		}
	}
	return i, n
}

func anchored(k syntax.TokenKind) bool {
	return k != syntax.TokNL && k != syntax.TokComma && k != syntax.TokColon && k != syntax.TokBOF
}

// importRange is the range of token indexes of the import block, or an empty one.
func importRange(f *syntax.File) (lo, hi int) {
	lo, hi = len(f.Tokens), -1
	for _, imp := range f.Imports {
		lo, hi = min(lo, int(imp.First())), max(hi, int(imp.Last()))
	}
	return lo, hi
}

func trimLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Join(lines, "\n")
}

// TestExamplesKeepTheirMeaning checks the §1 guarantees on every example.
func TestExamplesKeepTheirMeaning(t *testing.T) {
	for _, ex := range exampleFiles(t) {
		in := parse(t, ex.path, ex.data)
		checkFormatted(t, ex.path, in, mustFile(t, in.file))
	}
}

// commentOf is the text of a comment trivia, doc spacing aside.
func commentOf(f *syntax.File, tr syntax.Trivia) (string, bool) {
	s := trimLines(string(f.Src.Content[tr.Start:tr.End]))
	switch tr.Kind {
	case syntax.TriviaLineComment, syntax.TriviaBlockComment:
		return s, true
	case syntax.TriviaDocComment:
		if rest, ok := strings.CutPrefix(s, "/// "); ok {
			return "///" + rest, true
		}
		return s, true
	default:
		return "", false
	}
}
