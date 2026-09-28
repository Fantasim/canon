package i18n

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Result is one package's i18n state (VIEWMODEL.md §12.10).
type Result struct {
	Catalogue *Catalogue
	Languages map[string]*Language
}

// Language is one non-source project language's state for a package (I18N.md §5, §6).
type Language struct {
	Files   []string // the package's translation files for it, path order (F1)
	Missing int      // catalogue keys without a non-empty translation
	Texts   map[string]string
}

// Check builds every package's catalogue and checks its translation files (I18N.md §4, §5).
func Check(prog *check.Program, proj *project.Project, bags check.Bags, emitsView map[string]bool) map[string]*Result {
	if prog == nil || proj == nil || bags == nil || len(proj.Languages) == 0 {
		return nil
	}
	out := make(map[string]*Result, len(prog.Packages))
	for _, pkg := range prog.Packages {
		ctx := pkgCtx{info: prog.Info, proj: proj, bag: bags[pkg.Path], studioPath: proj.Studio.Path, emitsView: emitsView[pkg.Path]}
		out[pkg.Path] = checkPackage(pkg, ctx)
	}
	return out
}

// pkgCtx is the fixed context Check builds one package's Result with.
type pkgCtx struct {
	info       *check.Info
	proj       *project.Project
	bag        *diag.Bag
	studioPath string
	emitsView  bool
}

// checkPackage builds pkg's catalogue and checks its translation files.
func checkPackage(pkg *check.Package, ctx pkgCtx) *Result {
	r := &Result{Catalogue: build(pkg, ctx.info, ctx.studioPath), Languages: map[string]*Language{}}
	for _, lang := range ctx.proj.Languages[1:] {
		r.Languages[lang] = &Language{Texts: map[string]string{}}
	}
	seen := map[string]map[string]source.Span{}
	for _, f := range pkg.Files {
		if f.FileKind == syntax.FileTranslation {
			checkFile(f, ctx.proj, r, ctx.bag, seen)
		}
	}
	reportMissing(pkg, ctx.proj.Languages[1:], r, ctx.bag, ctx.emitsView)
	return r
}

// checkFile validates f's language (E1704, E1706) and, when valid, its entries.
func checkFile(f *syntax.File, proj *project.Project, r *Result, bag *diag.Bag, seen map[string]map[string]source.Span) {
	if f.Lang == nil {
		return // a syntax error already reported the missing language
	}
	lang := f.Lang.Name
	if !slices.Contains(proj.Languages, lang) {
		diag.E1704.At(f.Span(f.Lang), lang, proj.Languages).Report(bag)
		return
	}
	if lang == proj.Languages[0] {
		diag.E1706.At(f.Span(f.Lang), lang).Report(bag)
		return
	}
	l := r.Languages[lang]
	l.Files = append(l.Files, f.Src.Path)
	if seen[lang] == nil {
		seen[lang] = map[string]source.Span{}
	}
	ctx := entryCtx{f: f, cat: r.Catalogue, lang: l, bag: bag, seen: seen[lang]}
	for _, e := range f.Entries {
		checkEntry(ctx, e)
	}
}

// entryCtx is the fixed context for checking one translation file's entries (I18N.md §4).
type entryCtx struct {
	f    *syntax.File
	cat  *Catalogue
	lang *Language
	bag  *diag.Bag
	seen map[string]source.Span
}

// checkEntry checks one entry's key (E1702, E1705) and, when it names a catalogue key, its text.
func checkEntry(ctx entryCtx, e *syntax.TranslationEntry) {
	if e.Key == nil {
		return
	}
	key := syntax.Qualified(e.Key)
	span := ctx.f.Span(e.Key)
	if first, dup := ctx.seen[key]; dup {
		diag.E1705.At(span, key, first).Report(ctx.bag)
	} else {
		ctx.seen[key] = span
	}
	res := ctx.cat.Resolve(key)
	switch {
	case res.Found:
		checkText(ctx, e, key, res.Entry)
	case res.NoLetter:
		diag.E1702.AtNoLetter(span, key, ctx.cat.Package).Report(ctx.bag)
	case res.FormHint != "":
		diag.E1702.AtForm(span, key, ctx.cat.Package, res.FormHint).Report(ctx.bag)
	default:
		diag.E1702.AtPlain(span, key, ctx.cat.Package).Report(ctx.bag)
	}
}

// checkText applies F5 (empty counts as missing) and F6 (a plain translation cannot interpolate).
func checkText(ctx entryCtx, e *syntax.TranslationEntry, key string, entry Entry) {
	if literalRuns(e.Text) == "" && !hasInterp(e.Text) {
		return
	}
	if entry.Kind == Plain && hasInterp(e.Text) {
		diag.E1707.At(ctx.f.Span(firstInterp(e.Text)), key).Report(ctx.bag)
		return
	}
	ctx.lang.Texts[key] = sourceText(ctx.f, e.Text)
}
