package ir

// TextDecoders are the @text fns the cpp types-mode emit writes Decode<Fn>File of, in source order; none in another mode (DECISIONS 363).
func (pl *CppNamePlan) TextDecoders() []*ExportFn {
	return TextDecoded(pl.p, pl.e)
}

// TextFile is <Fn>File, <Fn> the fn's C++ name (FnName): what Decode<Fn>File's errors name (DECISIONS 340, 363).
func (pl *CppNamePlan) TextFile(fn *ExportFn) string {
	return pl.FnName(fn) + goTextFileSuffix
}

// TextDecoder is Decode<Fn>File, in the package namespace (DECISIONS 363).
func (pl *CppNamePlan) TextDecoder(fn *ExportFn) string {
	return goPublicDecode + pl.TextFile(fn)
}

// declareTextDecoders declares each Decode<Fn>File in the namespace, which other packages emitted into it share: a name it meets is E8005 (DECISIONS 363).
func (pl *CppNamePlan) declareTextDecoders() {
	for _, fn := range pl.TextDecoders() {
		pl.shareNS(pl.TextDecoder(fn), pl.p.Name+qnameSep+fn.Name, fn)
	}
}

// TSTextNames are a decoded @text fn's TypeScript functions, decode<Fn>File and parse<Fn>File, and <Fn>File, what their errors name (DECISIONS 363).
type TSTextNames struct {
	Decode, Parse, File string
}

// TSTextFunctions are the names of fn's TypeScript decoders, <Fn> the UpperCamel of its TypeScript name.
func TSTextFunctions(fn *ExportFn) TSTextNames {
	file := cppUpperCamel(tsEffective(fn.TS, tsLowerCamel(fn.Name))) + goTextFileSuffix
	return TSTextNames{Decode: tsDecodePrefix + file, Parse: tsParsePrefix + file, File: file}
}

// declareTextDecoders declares each decoded @text fn's decode<Fn>File and parse<Fn>File: a name it meets is E8005 (DECISIONS 363).
func (pl *tsNamePlan) declareTextDecoders(mod *nameScope) {
	for _, fn := range TextDecoded(pl.p, pl.e) {
		n := TSTextFunctions(fn)
		pl.declare(mod, n.Decode, pl.fnOrigin(fn), fn)
		pl.declare(mod, n.Parse, pl.fnOrigin(fn), fn)
	}
}
