package check

import (
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// fieldAnnotations resolves a field's annotations into its wire mapping, typeErr after a finding in its type (WIRE.md §4).
func (c *checker) fieldAnnotations(env *env, f *types.Field, fd *syntax.FieldDecl, wireCase string, typeErr bool) {
	if why, ok := c.deprecation(fd.Annotations); ok {
		f.Deprecated = &types.Deprecation{Why: why}
	}
	f.Stable = annotation(fd.Annotations, annotStable) != nil
	f.Wire = ConvertCase(f.Name, wireCase)
	f.WirePath = []string{f.Wire}
	j := annotation(fd.Annotations, annotJSON)
	if j == nil {
		return
	}
	if w, ok := c.annString(positional(j)); ok {
		f.Wire, f.WirePath = w, []string{w}
	}
	c.jsonPath(env, f, j)
	c.jsonForms(env, f, j, typeErr)
}

// jsonPath is `@json(path: "a.b")`: two or more non-empty segments, none starting with $.
func (c *checker) jsonPath(env *env, f *types.Field, j *syntax.Annotation) {
	v := named(j, jsonPath)
	p, ok := c.annString(v)
	if !ok {
		return
	}
	segs := strings.Split(p, dot)
	if len(segs) < minPathSegments || !cleanSegments(segs) {
		c.report(env, diag.E3316.AtPath(env.span(v), p))
		return
	}
	f.WirePath = segs
	f.Wire = segs[len(segs)-1]
}

func cleanSegments(segs []string) bool {
	for _, s := range segs {
		if s == "" || strings.HasPrefix(s, dollar) {
			return false
		}
	}
	return true
}

// jsonForms applies inline, int, bits, unit and none, judged on a type it fits, never after a type error (TYPES.md §1).
func (c *checker) jsonForms(env *env, f *types.Field, j *syntax.Annotation, typeErr bool) {
	if typeErr || f.Type.Base().Kind() == types.Error {
		c.untypedForms(env, f, j)
		return
	}
	if a := flagArg(j, jsonInline); a != nil {
		f.Inline = true
		if f.Type.Base().Kind() != types.Variant {
			c.report(env, diag.E3316.AtForm(env.span(a), jsonInline, f.Type))
		}
	}
	if a := flagArg(j, jsonInt); a != nil {
		f.Enc = types.EncInt
		if !holds(f.Type, types.Bool) {
			c.report(env, diag.E3316.AtForm(env.span(a), jsonInt, f.Type))
		}
	}
	if a := flagArg(j, jsonBits); a != nil {
		f.Enc = types.EncBits
		c.checkBits(env, f, a)
	}
	if v := named(j, jsonUnit); v != nil {
		f.Unit = unitOf(symbol(v))
		if !holds(f.Type, types.Duration) {
			c.report(env, diag.E3316.AtForm(env.span(v), jsonUnit, f.Type))
		}
	}
	if v := named(j, jsonNone); v != nil {
		c.jsonNone(env, f, v)
	}
	c.jsonPairs(env, f, j)
}

// jsonNone is `@json(none: x)`: only on an optional, x a marker (E3316); a marker holding a
// lexer error is unknown, and only its field's type is judged (DECISIONS 215).
func (c *checker) jsonNone(env *env, f *types.Field, v syntax.AnnValue) {
	known := !c.holdsLexError(v)
	if known {
		f.NoneWire = noneWire(v)
	}
	if f.Type.Base().Kind() != types.Optional || known && f.NoneWire == nil {
		c.report(env, diag.E3316.AtForm(env.span(v), jsonNone, f.Type))
	}
}

// untypedForms applies the forms of a field of the error type, judging only pairs: templates (WIRE.md §5.14).
func (c *checker) untypedForms(env *env, f *types.Field, j *syntax.Annotation) {
	f.Inline = flagArg(j, jsonInline) != nil
	if flagArg(j, jsonInt) != nil {
		f.Enc = types.EncInt
	}
	if flagArg(j, jsonBits) != nil {
		f.Enc = types.EncBits
	}
	if v := named(j, jsonUnit); v != nil {
		f.Unit = unitOf(symbol(v))
	}
	if v := named(j, jsonNone); v != nil && !c.holdsLexError(v) {
		f.NoneWire = noneWire(v)
	}
	if list, ok := named(j, jsonPairsName).(*syntax.AnnotationList); ok {
		c.pairKeys(env, list)
	}
}

// holds reports t contains k outside named types: in T?, [T], a map value, a dependent's arms (WIRE.md §4.1; DECISIONS 117).
func holds(t types.Type, k types.Kind) bool {
	switch x := t.Base().(type) {
	case *types.OptionalType:
		return holds(x.Elem, k)
	case *types.ListType:
		return holds(x.Elem, k)
	case *types.MapType:
		return holds(x.Value, k)
	case *types.DepMapType:
		return holds(x.Value, k)
	case *types.TypeAppType:
		return k != types.Error && armsHold(x.Fn, k) // an arm in error is its declaration's finding, not the field's
	}
	return t.Base().Kind() == k
}

// armsHold reports a body or arm holding k, one in error a wildcard that does (DECISIONS 117; TYPES.md §1).
func armsHold(fn *types.TypeFunc, k types.Kind) bool {
	if fn.Body != nil {
		return holds(fn.Body, k) || holds(fn.Body, types.Error)
	}
	if fn.Scrutinee == nil {
		return true
	}
	return slices.ContainsFunc(fn.Arms, func(a *types.TypeArm) bool {
		return holds(a.Result, k) || holds(a.Result, types.Error)
	})
}

func unitOf(sym string) types.Unit {
	for u, name := range unitSymbols {
		if name == sym {
			return types.Unit(u)
		}
	}
	return types.UnitMs
}

// checkBits is `@json(bits)`: `[E]` or `[E]?`, E with `@codes` whose every code is a power of
// two from 1 to 2^62.
func (c *checker) checkBits(env *env, f *types.Field, a *syntax.AnnotationArg) {
	t := f.Type.Base()
	if o, ok := t.(*types.OptionalType); ok {
		t = o.Elem.Base()
	}
	l, ok := t.(*types.ListType)
	if !ok {
		c.report(env, diag.E3316.AtForm(env.span(a), jsonBits, f.Type))
		return
	}
	e, ok := l.Elem.Base().(*types.EnumType)
	if !ok || e.Codes == nil {
		c.report(env, diag.E3316.AtForm(env.span(a), jsonBits, f.Type))
		return
	}
	c.completeEnum(c.typeObjects[e], e)
	for _, m := range e.Members {
		if !m.HasCode {
			continue // no code: its E3201 or lexer error is the one finding (TYPES.md §1)
		}
		if m.Code < 1 || m.Code > maxBit || m.Code&(m.Code-1) != 0 {
			c.report(env, diag.E3316.AtBits(env.span(a), e.String()))
			return
		}
	}
}

// noneWire is the compact JSON of a `none:` marker: an integer, a float, a string, true,
// false, {} or []; nil for anything else.
func noneWire(v syntax.AnnValue) []byte {
	switch x := v.(type) {
	case *syntax.IntLit:
		return []byte(x.Value.String())
	case *syntax.FloatLit:
		return []byte(types.FloatText(floatOf(x), bits64))
	case *syntax.StringLit:
		return diag.AppendJSONString(nil, constText(x))
	case *syntax.BoolLit:
		return []byte(strconv.FormatBool(x.Value))
	case *syntax.BraceLit:
		return []byte(emptyObject)
	case *syntax.AnnotationList:
		if len(x.Items) == 0 {
			return []byte(emptyArray)
		}
	}
	return nil
}

// jsonPairs is `@json(pairs: [k, v])` (WIRE.md §5.14).
func (c *checker) jsonPairs(env *env, f *types.Field, j *syntax.Annotation) {
	v := named(j, jsonPairsName)
	list, ok := v.(*syntax.AnnotationList)
	if !ok {
		return
	}
	keys, known, ok := c.pairKeys(env, list)
	if !ok {
		return
	}
	slots, ok := pairSlots(f.Type)
	if !ok {
		c.report(env, diag.E3316.AtPairsBound(env.span(v), f.Name))
		return
	}
	if !c.pairsElem(f.Type.(*types.Refined).Of) {
		c.report(env, diag.E3316.AtForm(env.span(j), jsonPairsName, f.Type))
		return
	}
	if known {
		f.Pairs = &types.Pairs{Keys: keys, Slots: slots}
	}
}

// pairKeys are the two templates of `pairs:`, each with one slot and distinct (E3316, then ok is
// false); not known when one holds a lexer error, whose text is made up (DECISIONS 215).
func (c *checker) pairKeys(env *env, list *syntax.AnnotationList) (keys [pairCount]string, known, ok bool) {
	if len(list.Items) != pairCount {
		c.report(env, diag.E3316.AtPairsTemplate(env.span(list), ""))
		return keys, false, false
	}
	if c.holdsLexError(list) {
		return keys, false, true
	}
	for i := range keys {
		s, isStr := c.templateText(list.Items[i])
		if !isStr || strings.Count(s, pairSlot) != 1 {
			c.report(env, diag.E3316.AtPairsTemplate(env.span(list.Items[i]), s))
			return keys, false, false
		}
		keys[i] = s
	}
	if keys[0] == keys[1] {
		c.report(env, diag.E3316.AtPairsTemplate(env.span(list), keys[1]))
		return keys, false, false
	}
	return keys, true, true
}

// pairsElem reports an element record of two present scalar fields, no input, no `$` key; one containing itself is E3022's (WIRE.md §4.1).
func (c *checker) pairsElem(list types.Type) bool {
	rec, ok := list.Base().(*types.ListType).Elem.Base().(*types.RecordType)
	if !ok {
		return false
	}
	c.completeRecord(rec)
	if reaches(rec, rec, map[*types.RecordType]bool{}) {
		return true
	}
	if len(rec.Fields) != pairCount {
		return false
	}
	for _, f := range rec.Fields {
		if f.Input != nil || !scalarWire(c.fieldType(f)) {
			return false
		}
	}
	return !slices.ContainsFunc(rec.Methods, func(m *types.Method) bool { return m.Export && !c.translated(m) })
}

// scalarWire reports a type whose wire form is one JSON scalar, an optional excluded, the error type included (TYPES.md §1).
func scalarWire(t types.Type) bool {
	switch t.Base().Kind() {
	case types.Bool, types.Int, types.Float, types.String, types.Duration, types.Enum, types.Ref,
		types.LitUnion, types.Error:
		return true
	default:
		return false
	}
}

// translated reports an export fn with a non-finite parameter, which has no `$` key (WIRE.md §5.11).
func (c *checker) translated(m *types.Method) bool {
	return slices.ContainsFunc(m.Type.Params, func(p types.Type) bool {
		switch x := p.Base().(type) {
		case *types.RefType:
			coll := c.coll(x)
			return coll == nil || coll.KeyedBy != nil || coll.Local && coll.Kind == types.CollLet // DECISIONS 296
		default:
			k := p.Base().Kind()
			return k != types.Bool && k != types.Enum
		}
	})
}

// pairSlots is the upper bound of a plain list's length refinement: N slots.
func pairSlots(t types.Type) (int, bool) {
	r, ok := t.(*types.Refined)
	if !ok || r.Range == nil || !r.Range.HasHi {
		return 0, false
	}
	if l, isList := r.Of.Base().(*types.ListType); !isList || l.KeyedBy != nil {
		return 0, false
	}
	n := r.Range.Hi.I
	if !r.Range.HiIncluded {
		n--
	}
	return int(n), n >= 0
}

// ConvertCase is a field's default wire name under `@json(case:)` (WIRE.md §5.5.2).
func ConvertCase(name, wireCase string) string {
	var join string
	upper := false
	switch wireCase {
	case caseSnake:
		join = underscore
	case caseKebab:
		join = hyphen
	case caseUpperSnake:
		join, upper = underscore, true
	default:
		return name
	}
	words := splitWords(name)
	for i, w := range words {
		if upper {
			words[i] = strings.ToUpper(w)
		} else {
			words[i] = strings.ToLower(w)
		}
	}
	return strings.Join(words, join)
}

// splitWords splits a Canon name at `_` and at case boundaries; digits stay with the word
// before them.
func splitWords(name string) []string {
	var words []string
	cur := ""
	for i := range len(name) {
		ch := name[i]
		if ch == '_' {
			words, cur = appendWord(words, cur), ""
			continue
		}
		if i > 0 && boundary(name, i) {
			words, cur = appendWord(words, cur), ""
		}
		cur += name[i : i+1]
	}
	return appendWord(words, cur)
}

func appendWord(words []string, w string) []string {
	if w == "" {
		return words
	}
	return append(words, w)
}

// boundary is WIRE.md §5.5.2 rule 2 at name[i].
func boundary(name string, i int) bool {
	prev, ch := name[i-1], name[i]
	if !isUpper(ch) {
		return false
	}
	if isLower(prev) || isDigit(prev) {
		return true
	}
	return isUpper(prev) && i+1 < len(name) && isLower(name[i+1])
}

func isUpper(b byte) bool { return 'A' <= b && b <= 'Z' }
func isLower(b byte) bool { return 'a' <= b && b <= 'z' }
func isDigit(b byte) bool { return '0' <= b && b <= '9' }

// recordCase is the `@json(case:)` of a record header, "" without one.
func recordCase(anns []*syntax.Annotation) string {
	return symbol(named(annotation(anns, annotJSON), jsonCase))
}

// caseWireCase is the case of a case's fields: its own `case:`, else its variant's.
func (c *checker) caseWireCase(ct *types.CaseType) string {
	vc := c.caseDecls[ct]
	if own := recordCase(vc.Annotations); own != "" {
		return own
	}
	return c.variantCases[ct.Variant]
}

// enumAnnotations is `@codes(T)` and `@json(codes)` on an enum header; the latter needs the former (GRAMMAR.md §8.3).
func (c *checker) enumAnnotations(e *types.EnumType, anns []*syntax.Annotation) {
	if codes := annotation(anns, annotCodes); codes != nil {
		if b, ok := c.universe[symbol(firstArg(codes))]; ok && b.typ != nil {
			if basic, isBasic := b.typ.(types.Basic); isBasic && basic.K == types.Int {
				e.Codes = &basic
			}
		}
	}
	e.WireCodes = e.Codes != nil && flag(annotation(anns, annotJSON), jsonCodes)
}

// enumMember is a member's wire value and code (WIRE.md §5.3, TYPES.md §8.1).
func (c *checker) enumMember(env *env, e *types.EnumType, i int, m *syntax.EnumMember) *types.Member {
	mem := &types.Member{Name: m.Name.Name, Wire: m.Name.Name, Index: i, Doc: docText(m.Doc)}
	mem.Retired = m.Mods != nil && m.Mods.Retired.Valid()
	if why, ok := c.deprecation(m.Annotations); ok {
		mem.Deprecated = &types.Deprecation{Why: why}
	}
	if w, ok := c.annString(positional(annotation(m.Annotations, annotJSON))); ok {
		mem.Wire = w
	}
	if m.Value != nil && c.lexError(m.Value) {
		c.info.Types[m.Value] = types.ErrorType
		return mem
	}
	switch v := m.Value.(type) {
	case syntax.StrLit:
		c.info.Types[v] = types.StringType
		mem.Wire = constText(v)
		if e.Codes != nil {
			c.report(env, diag.E3002.At(env.span(v), *e.Codes, types.StringType))
		}
	case *syntax.IntLit:
		c.memberCode(env, e, mem, v)
	case nil:
		if e.Codes != nil {
			c.report(env, diag.E3201.At(env.span(m.Name), rawValue(noneWord), *e.Codes))
		}
	}
	return mem
}

func (c *checker) memberCode(env *env, e *types.EnumType, mem *types.Member, v *syntax.IntLit) {
	c.info.Types[v] = types.IntType
	if e.Codes == nil {
		c.report(env, diag.E3002.At(env.span(v), types.StringType, types.IntType))
		return
	}
	lo, hi, _ := e.Codes.Limits()
	if !v.Value.IsInt64() || v.Value.Int64() < lo || v.Value.Int64() > hi {
		c.report(env, diag.E3201.At(env.span(v), rawValue(v.Value.String()), *e.Codes))
		return
	}
	mem.Code, mem.HasCode = v.Value.Int64(), true
}

// variantAnnotations is `@json(tag:)` and `@json(case:)` on a variant header.
func (c *checker) variantAnnotations(v *types.VariantType, anns []*syntax.Annotation) {
	j := annotation(anns, annotJSON)
	if tag, ok := c.annString(named(j, jsonTag)); ok {
		v.Tag = tag
	}
	c.variantCases[v] = symbol(named(j, jsonCase))
}

// caseAnnotations is a case's `@json("w")` and `@deprecated`.
func (c *checker) caseAnnotations(ct *types.CaseType, vc *syntax.VariantCase) {
	c.caseDecls[ct] = vc
	if w, ok := c.annString(positional(annotation(vc.Annotations, annotJSON))); ok {
		ct.Wire = w
	}
	if why, ok := c.deprecation(vc.Annotations); ok {
		ct.Deprecated = &types.Deprecation{Why: why}
	}
}
