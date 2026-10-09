package ir

import "slices"

// OpenEnum reports an enum the go types-mode emit of its package opens (DECISIONS 339): e's own `open` for an enum of p, else that of the go emit of its package as p imports it.
func OpenEnum(p *Package, e *Emit, en *Enum) bool {
	owner := e
	if en.Pkg != p.Name {
		owner = ownerEmit(p, en.Pkg, TargetGo)
	}
	return owner != nil && owner.Target == TargetGo && owner.Mode == ModeTypes && slices.Contains(owner.Open, en.Name)
}

// enumMethods are the fixed methods of an enum of the package: Known too for an opened one, Code with @codes (CODEGEN.md §5.2, DECISIONS 339).
func (pl *GoNamePlan) enumMethods(e *Enum) []string {
	open := OpenEnum(pl.p, pl.e, e)
	switch {
	case open && e.Codes != nil:
		return goOpenCodesMethods
	case open:
		return goOpenEnumMethods
	case e.Codes != nil:
		return goCodesEnumMethods
	}
	return goEnumMethods
}

// TextResult is what a @text fn's file holds: its result, without a maybe-file's `?` (CODEGEN.md §2.9).
func TextResult(fn *ExportFn) TypeRef {
	t, _ := goUnwrap(fn.Result)
	return t
}

// TextDecoder is Decode<Fn>File, a decoded @text fn's public decoder (DECISIONS 340).
func (pl *GoNamePlan) TextDecoder(fn *ExportFn) string { return goPublicDecode + pl.TextFile(fn) }

// TextFile is <Fn>File, <Fn> the fn's generated UpperCamel name: what its decoder's errors name, as Decode<T>'s name <T> (DECISIONS 340).
func (pl *GoNamePlan) TextFile(fn *ExportFn) string {
	return goExported(fn.Go, fn.Name) + goTextFileSuffix
}

// TextDecoders are the @text fns a types-mode emit writes Decode<Fn>File of; none in another mode.
func (pl *GoNamePlan) TextDecoders() []*ExportFn {
	if pl.e.Mode != ModeTypes {
		return nil
	}
	return TextDecoded(pl.p, pl.e)
}

// declareTextDecoders declares Decode<Fn>File of each decoded @text fn, after the records' and variants' decoders: a name it meets is E8005 (DECISIONS 340).
func (pl *GoNamePlan) declareTextDecoders(top *nameScope) {
	for _, fn := range pl.TextDecoders() {
		pl.declare(top, pl.TextDecoder(fn), pl.p.Name+qnameSep+fn.Name, fn)
	}
}

// textHolds reports a decoded @text fn's result holding, at any depth, a type the predicate accepts.
func (pl *GoNamePlan) textHolds(is func(TypeRef) bool) bool {
	found := false
	for _, fn := range pl.TextDecoders() {
		walkTypeRef(TextResult(fn), func(t TypeRef) { found = found || is(t) })
	}
	return found
}
