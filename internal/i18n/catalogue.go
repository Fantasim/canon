package i18n

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// Entry is one catalogue key, its source text and its kind (I18N.md §3.3).
type Entry struct {
	Key  string
	Text string
	Kind Kind
}

// Catalogue is a package's key catalogue, in catalogue order (I18N.md §3, K10).
type Catalogue struct {
	Package string
	Entries []Entry

	byKey       map[string]int
	shadow      map[string]bool   // keys that exist but whose source text has no letter (F4 noLetter)
	form        map[string]string // a K4-wrong key -> the key its writer meant (F4 form)
	names       map[string]bool   // every top-level declaration's name, broken or not (F4 silent)
	broken      map[string]bool   // top-level declarations left out of the catalogue because they are broken (F4 silent)
	brokenViews map[string]bool   // exact keys a broken view alone supplied (VIEWMODEL.md J4, F4 silent)
	unparsed    bool              // a source file of the package did not fully parse (F4 silent)
}

// Lookup is the entry named key, when key is in the catalogue.
func (c *Catalogue) Lookup(key string) (Entry, bool) {
	i, ok := c.byKey[key]
	if !ok {
		return Entry{}, false
	}
	return c.Entries[i], true
}

// Resolution is how a translation key relates to the catalogue (I18N.md F4).
type Resolution struct {
	Entry    Entry
	Found    bool
	Silent   bool   // a cascade, not a defect: report nothing (F4, TYPES.md §1)
	NoLetter bool   // key exists but its source text has no letter
	FormHint string // the key its writer meant, when only its K4 form differs
}

// Resolve classifies key against the catalogue for E1702 (I18N.md F4).
func (c *Catalogue) Resolve(key string) Resolution {
	if e, ok := c.Lookup(key); ok {
		return Resolution{Entry: e, Found: true}
	}
	if c.silent(key) {
		return Resolution{Silent: true}
	}
	if c.shadow[key] {
		return Resolution{NoLetter: true}
	}
	if hint, ok := c.form[key]; ok {
		return Resolution{FormHint: hint}
	}
	return Resolution{}
}

// silent is F4's cascade carve-out: a broken declaration, a key only a broken view would supply,
// or, with an unparsed file, an unknown first segment (a check.<n> key counts as unknown then
// too, F4).
func (c *Catalogue) silent(key string) bool {
	first, _, _ := strings.Cut(key, dot)
	if c.broken[first] || c.brokenViews[key] {
		return true
	}
	return c.unparsed && !c.names[first]
}

// builder accumulates a catalogue's entries in the order they are found (Build sorts them).
type builder struct {
	pkg    *check.Package
	info   *check.Info
	views  map[check.Object]viewEntry // the let targets: a define-table view (I18N.md K "v.title")
	byT    map[types.Type]viewEntry   // record, variant, case and enum targets
	files  map[types.Type]*syntax.File
	inline map[*types.Field]inlineLabel // a case field reached by inlining (I18N.md K7)
	viewOf map[string]*syntax.ViewDecl  // keys added only because a view supplies them (VIEWMODEL.md J4)
	cat    *Catalogue
}

// build is pkg's key catalogue (I18N.md K1, K3): read as if every view were intact, then a
// broken one's own keys are dropped.
func build(pkg *check.Package, info *check.Info, studioPath string) *Catalogue {
	b := newBuilder(pkg, info)
	if pkg.Path == studioPath {
		b.studioMenu()
		b.studioUnits()
	} else {
		b.packageEntries()
	}
	b.stripBrokenViewKeys()
	b.finish()
	return b.cat
}

// newBuilder indexes pkg's first views by target and by type.
func newBuilder(pkg *check.Package, info *check.Info) *builder {
	views := firstViews(pkg, info)
	names, broken := topLevelState(pkg, info)
	return &builder{
		pkg: pkg, info: info, views: views, byT: viewsByType(views), files: typeFiles(pkg),
		cat: &Catalogue{
			Package: pkg.Path, shadow: map[string]bool{}, form: map[string]string{},
			names: names, broken: broken, brokenViews: map[string]bool{}, unparsed: hasBadNode(pkg),
		},
	}
}

// topLevelState is pkg's top-level declaration names, and which of them are broken and so left
// out of the catalogue (VIEWMODEL.md J4, F4).
func topLevelState(pkg *check.Package, info *check.Info) (names, broken map[string]bool) {
	names, broken = map[string]bool{}, map[string]bool{}
	for _, o := range pkg.Decls {
		if o.Name() == "" {
			continue
		}
		names[o.Name()] = true
		if info.Broken[o] {
			broken[o.Name()] = true
		}
	}
	return names, broken
}

// hasBadNode reports a source file of pkg holding a node the parser recovered from (I18N.md F4:
// a file that lost part of its syntax), read from the AST itself: bag.Findings() is sorted, deduplicated
// and truncated (API.md F2, F7) and could drop the one finding this decides on.
func hasBadNode(pkg *check.Package) bool {
	for _, f := range pkg.Files {
		if f.FileKind == syntax.FileSource && badNodeIn(f) {
			return true
		}
	}
	return false
}

// badNodeIn reports f holding a BadDecl, BadStmt, BadExpr or BadType.
func badNodeIn(f *syntax.File) bool {
	found := false
	syntax.Inspect(f, func(n syntax.Node) bool {
		switch n.(type) {
		case *syntax.BadDecl, *syntax.BadStmt, *syntax.BadExpr, *syntax.BadType:
			found = true
		}
		return !found
	})
	return found
}

// typeFiles maps each of pkg's type-name objects to its declaring file.
func typeFiles(pkg *check.Package) map[types.Type]*syntax.File {
	files := map[types.Type]*syntax.File{}
	for _, o := range pkg.Decls {
		if o.Kind() == check.ObjTypeName {
			files[o.Type()] = o.File()
		}
	}
	return files
}

// viewsByType is views' record, variant, case and enum targets, keyed by type.
func viewsByType(views map[check.Object]viewEntry) map[types.Type]viewEntry {
	byT := map[types.Type]viewEntry{}
	for o, v := range views { //canon:unordered each key set once, from its own object
		if o.Kind() != check.ObjLet {
			byT[o.Type()] = v
		}
	}
	return byT
}

// packageEntries adds a non-studio package's types, public lets and named checks (I18N.md K1),
// a broken declaration left out (VIEWMODEL.md J4).
func (b *builder) packageEntries() {
	reach := reachableTypes(b.pkg, b.info)
	b.inline = inlineLabels(reach, b.byT, b.info)
	for _, t := range reach {
		b.typeEntries(t)
	}
	for _, o := range b.pkg.Decls {
		if o.Kind() == check.ObjLet && !b.info.Broken[o] {
			b.letEntries(o)
		}
	}
	for _, o := range b.pkg.Decls {
		if o.Kind() == check.ObjCheck && !b.info.Broken[o] {
			b.packageCheck(o)
		}
	}
}

// finish sorts the entries by byte order (K10) and builds the lookup index.
func (b *builder) finish() {
	slices.SortFunc(b.cat.Entries, func(a, c Entry) int {
		if a.Key < c.Key {
			return -1
		}
		if a.Key > c.Key {
			return 1
		}
		return 0
	})
	b.cat.byKey = make(map[string]int, len(b.cat.Entries))
	for i, e := range b.cat.Entries {
		b.cat.byKey[e.Key] = i
	}
}

// add records text at realKey when ok (I18N.md L6: translatable); altKey, when different, is
// registered as the wrong K4 form so a translator using it gets a hint (F4).
func (b *builder) add(realKey, altKey, text string, kind Kind, ok bool) {
	if altKey != "" && altKey != realKey {
		b.cat.form[altKey] = realKey
	}
	if !ok {
		b.shadow(realKey)
		return
	}
	b.cat.Entries = append(b.cat.Entries, Entry{Key: realKey, Text: text, Kind: kind})
}

func (b *builder) shadow(key string) { b.cat.shadow[key] = true }

// addText resolves s's source text (via f, for a template's braces) and adds it at realKey;
// translatability is judged on s's literal runs, never on an interpolation's expression (L6).
func (b *builder) addText(realKey, altKey string, f *syntax.File, s syntax.StrLit, kind Kind) {
	if s == nil {
		return
	}
	b.add(realKey, altKey, SourceText(f, s), kind, translatable(literalRuns(s)))
}

// addPlain adds an already-normalized plain text (a doc comment, a deprecation reason).
func (b *builder) addPlain(realKey, altKey, text string) {
	if text == "" {
		return
	}
	b.add(realKey, altKey, text, Plain, translatable(text))
}

// addViewText is addText with no K4-wrong form, remembering that view alone supplies realKey
// (nil view: as addText); every view-only key is written bare (title, a group or show id, …).
func (b *builder) addViewText(realKey string, f *syntax.File, s syntax.StrLit, kind Kind, view *syntax.ViewDecl) {
	if s != nil {
		b.tagView(realKey, view)
	}
	b.addText(realKey, "", f, s, kind)
}

// tagView records that view alone supplies key, judged once every entry is built (VIEWMODEL.md
// J4, I18N.md F4): a no-op for view nil.
func (b *builder) tagView(key string, view *syntax.ViewDecl) {
	if view == nil {
		return
	}
	if b.viewOf == nil {
		b.viewOf = map[string]*syntax.ViewDecl{}
	}
	b.viewOf[key] = view
}

// stripBrokenViewKeys drops every key tagView recorded whose view check.ViewBroken now judges
// broken (VIEWMODEL.md J4): the key leaves the catalogue and, unlike a broken declaration's,
// stays remembered by its own key so a stale translation of it stays silent, not E1702 (F4).
func (b *builder) stripBrokenViewKeys() {
	for key, d := range b.viewOf { //canon:unordered each key judged on its own recorded view, order-free
		if check.ViewBroken(b.info, d) {
			b.cat.brokenViews[key] = true
			delete(b.cat.shadow, key)
		}
	}
	if len(b.cat.brokenViews) == 0 {
		return
	}
	b.cat.Entries = slices.DeleteFunc(b.cat.Entries, func(e Entry) bool { return b.cat.brokenViews[e.Key] })
}
