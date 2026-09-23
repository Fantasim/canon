package syntax

import "github.com/fantasim/canonlang/internal/diag"

// file parses the whole file; its kind is decided by its first tokens (GRAMMAR.md §5.2).
func (p *parser) file(isProject bool) *File {
	f := &File{Src: p.src, Tokens: p.toks, FileKind: FileSource}
	switch {
	case p.at(KwProject):
		if !isProject {
			diag.E1011.AtOutside(p.span(p.pos, p.pos)).Report(p.bag)
		}
		p.projectFile(f)
	case isProject:
		diag.E1011.AtInside(p.span(p.pos, p.pos)).Report(p.bag)
		for !p.at(TokEOF) && (!p.at(KwProject) || !p.topSync()) {
			p.next()
		}
		p.projectFile(f)
	default:
		p.packageFile(f)
	}
	for !p.at(TokEOF) {
		p.next()
	}
	f.Bounds = Bounds{From: NoTok + 1, To: p.pos}
	return f
}

// projectFile is { DOC } "project" IDENT BraceList(projectItem) { NL } EOF (GRAMMAR.md §7).
func (p *parser) projectFile(f *File) {
	f.FileKind = FileProject
	if p.at(KwProject) {
		f.Project = p.projectDecl()
	}
	p.skipNL()
	if !p.at(TokEOF) {
		diag.E1011.AtInside(p.span(p.pos, p.pos)).Report(p.bag)
	}
}

// packageFile reads the package clause, then a layer, a translation or a source file.
func (p *parser) packageFile(f *File) {
	pkg := p.pos
	if p.at(KwPackage) {
		p.next()
		f.Package = p.qualified(p.packageSegment, isWord)
		p.endTop(pkg, f.Package != nil)
	} else {
		diag.E1127.AtPackage(p.span(p.pos, p.pos)).Report(p.bag)
	}
	switch {
	case p.at(KwLayer) && f.Package != nil:
		p.layerFile(f)
	case p.at(KwTranslation) && f.Package != nil:
		p.translationFile(f)
	default:
		if f.Package != nil {
			f.Doc = p.doc(pkg)
		}
		p.topItems(func() bool { return p.sourceItem(f) }, func(b Bounds) {
			f.Decls = append(f.Decls, &BadDecl{Bounds: b})
		})
	}
}

func (p *parser) packageSegment() *Ident { return p.ident(diag.KindPackageSegment) }

// topItems parses the items of a file, separated by NL (GRAMMAR.md §3.1).
func (p *parser) topItems(item func() bool, bad func(Bounds)) {
	for !p.at(TokEOF) {
		if p.accept(TokNL) != NoTok {
			continue
		}
		start := p.pos
		ok := item()
		if !p.endTop(start, ok) && bad != nil {
			bad(p.from(start))
		}
	}
}

// endTop ends a top-level item at NL or EOF; after an error it skips to the next item and
// clears it. It reports whether the item parsed.
func (p *parser) endTop(start Tok, ok bool) bool {
	if ok && !p.bail {
		switch {
		case p.at(TokNL) || p.at(TokEOF):
			p.accept(TokNL)
			return true
		case startsItem(p.kind()):
			diag.E1117.At(p.span(p.pos, p.pos)).Report(p.bag)
			return true
		}
		p.fail(expected(TokNL))
	}
	p.syncTop(start)
	p.bail = false
	return ok
}

// sourceItem is an import or a top-level declaration; an import after a declaration (E1127),
// a second package clause and a project declaration are read and left out of the tree.
func (p *parser) sourceItem(f *File) bool {
	if isModifier[p.kind()] && p.afterModifiers() == KwImport {
		p.dropMods(p.modifiers(), diag.KindImport)
	}
	switch p.kind() {
	case KwImport:
		late := len(f.Decls) > 0
		if late {
			diag.E1127.AtImports(p.span(p.pos, p.pos)).Report(p.bag)
		}
		imp := p.importDecl()
		if imp != nil && !late {
			f.Imports = append(f.Imports, imp)
		}
		return imp != nil
	case KwPackage:
		diag.E1127.AtSecondPackage(p.span(p.pos, p.pos)).Report(p.bag)
		p.next()
		return p.qualified(p.packageSegment, isWord) != nil
	case KwProject:
		diag.E1011.AtOutside(p.span(p.pos, p.pos)).Report(p.bag)
		return p.projectDecl() != nil
	default:
	}
	d := p.topDecl()
	if d != nil {
		f.Decls = append(f.Decls, d)
	}
	return d != nil
}

// afterModifiers is the kind of the first token after the modifiers at pos.
func (p *parser) afterModifiers() TokenKind {
	i := 0
	for isModifier[p.peek(i)] {
		i++
	}
	return p.peek(i)
}

// importDecl is "import" qualifiedIdent [ "as" IDENT ] [ BraceList( IDENT ) ].
func (p *parser) importDecl() *Import {
	start := p.pos
	p.next()
	imp := &Import{Path: p.qualified(p.packageSegment, isWord)}
	if imp.Path == nil {
		return nil
	}
	if p.accept(KwAs) != NoTok {
		if imp.Alias = p.ident(diag.KindImportAlias); imp.Alias == nil {
			return nil
		}
	}
	if p.at(TokLBrace) {
		imp.Braces = p.braceList(func() bool {
			id := p.ref()
			if id != nil {
				imp.Names = append(imp.Names, id)
			}
			return id != nil
		}, nil)
		if imp.Braces.Close == NoTok {
			return nil
		}
	}
	imp.Bounds = p.from(start)
	return imp
}

// layerFile is "layer" IDENT NL { amendDecl ( NL | EOF ) } (GRAMMAR.md §5.2, §5.7).
func (p *parser) layerFile(f *File) {
	f.FileKind = FileLayer
	start := p.next()
	f.Layer = p.ident(diag.KindLayer)
	p.endTop(start, f.Layer != nil)
	p.topItems(func() bool {
		if !p.at(KwAmend) {
			diag.E1127.AtLayer(p.span(p.pos, p.pos)).Report(p.bag)
			p.bail = true
			return false
		}
		a := p.amendBlock()
		if a != nil {
			f.Amends = append(f.Amends, a)
		}
		return a != nil
	}, nil)
}

// translationFile is "translation" IDENT NL { translationEntry ( NL | EOF ) }.
func (p *parser) translationFile(f *File) {
	f.FileKind = FileTranslation
	start := p.next()
	f.Lang = p.ref()
	p.endTop(start, f.Lang != nil)
	p.topItems(func() bool {
		if p.declaration() {
			diag.E1127.AtTranslation(p.span(p.pos, p.pos)).Report(p.bag)
			p.bail = true
			return false
		}
		e := p.translationEntry()
		if e != nil {
			f.Entries = append(f.Entries, e)
		}
		return e != nil
	}, nil)
}

// declaration reports a declaration where a translation key is expected: a declaration keyword,
// "import" or "package" not followed by "." or a string.
func (p *parser) declaration() bool {
	k := p.kind()
	if !declKeyword[k] && !topSyncKind[k] {
		return false
	}
	next := p.peek(1)
	return next != TokDot && !startsString[next]
}
