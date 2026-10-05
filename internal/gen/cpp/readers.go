package cppgen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// readerDecls declare this package's reader of each class of another package its loaders or decoders read, before the decoders calling them, in an unnamed namespace: two packages of one namespace may read one class (CODEGEN.md §2.7, §2.8; log-2026-10-06 "U3 review FAIL"). A reader no decoder calls, a case's, is maybe unused.
func (g *gen) readerDecls() {
	read := g.pl.Foreign().Read
	if len(read) == 0 {
		return
	}
	g.c.line(anonOpen)
	for _, c := range read {
		g.c.write(maybeUnused)
		if d, ok := c.(*ir.Dependent); ok {
			g.c.printf(dependentDeclFormat, g.pl.ReaderName(c), g.global(*d.Disc), g.typeName(d))
			continue
		}
		g.c.printf(readerDeclFormat, g.pl.ReaderName(c), g.readClassName(c))
	}
	g.c.line(anonClose)
	g.c.blank()
}

// written is h, refused unless its owner writes it: stage E refuses its use first (E8019 ForeignResolvedRef; log-2026-10-06 "U1 review" 3).
func (g *gen) written(h ir.CppHook) ir.CppHook {
	if !h.Written {
		g.malformed(unwrittenHook, g.at)
	}
	return h
}

// readClassName is the qualified class a reader fills.
func (g *gen) readClassName(c any) string {
	if cs, ok := c.(*ir.Case); ok {
		return g.caseName(g.pl.Foreign().VariantOf(cs), cs)
	}
	t, _ := c.(ir.Type)
	return g.typeName(t)
}

// readers define the readers, in first-reach order: each reads the class's wire as this package's decoders read its own classes, into locals named after the members, then builds the value through its owner's make hook (CODEGEN.md §2.8, §5.14, §7.6).
func (g *gen) readers() {
	u := g.pl.Foreign()
	if len(u.Read) == 0 {
		return
	}
	g.c.line(anonOpen)
	g.c.blank()
	defer func() {
		g.c.line(anonClose)
		g.c.blank()
	}()
	for _, c := range u.Read {
		prev := g.view
		leave := g.enter(g.readClassName(c))
		switch x := c.(type) {
		case *ir.Record:
			g.view = x.Pkg
			h := g.written(g.pl.RecordHook(x))
			g.readerOpen(c)
			g.readFields(class{rec: x}, x.Pkg, h.Name, h.Members)
		case *ir.Case:
			v := u.VariantOf(x)
			g.view = v.Pkg
			h := g.written(g.pl.CaseHook(v, x))
			g.readerOpen(c)
			g.readFields(class{variant: v, cs: x}, v.Pkg, h.CaseType, h.Members)
		case *ir.Variant:
			g.view = x.Pkg
			g.readerOpen(c)
			g.readVariant(x)
		case *ir.Dependent:
			g.view = x.Pkg
			g.dependentDecoder(x, g.pl.ReaderName(c))
			leave()
			g.view = prev
			continue
		}
		g.c.line(closeBrace)
		g.c.blank()
		leave()
		g.view = prev
	}
}

// readerOpen opens a reader's definition.
func (g *gen) readerOpen(c any) {
	g.c.printf(readerOpenFormat, g.pl.ReaderName(c), g.readClassName(c))
}

// readFields reads a record's or case's object into locals, then sets out from the hook of package pkg named hook: no value unless all of it decoded.
func (g *gen) readFields(c class, pkg, hook string, members []ir.CppHookMember) {
	prev := g.dst
	g.dst = ""
	defer func() { g.dst = prev }()
	fields, fns := c.shape()
	g.inlineFolds(c)
	g.checkKeys(1, sourceVar, g.wireKeys(g.objectKeys(c)), true)
	g.hookLocals(1, members, "")
	g.decodeBody(fields, fns)
	g.c.linef(1, notOkReturnLine)
	g.foreignEntryChecks(entrySite{depth: 1, pkg: pkg}, members)
	g.c.linef(1, assignFormat, outVar, g.makeCall(pkg, hook, movedLocals(members, "")))
	g.c.linef(1, returnTrue)
}

// readVariant reads the tag, then the case's fields from the same object into locals, and sets out from the case's hook (WIRE.md §5.6, CODEGEN.md §5.14).
func (g *gen) readVariant(v *ir.Variant) {
	if v.Tag == "" {
		g.fail(fmt.Errorf("%w: variant %s without a tag", ErrMalformed, v.Name))
		return
	}
	g.checkKeys(1, sourceVar, g.wireKeys([]string{v.Tag}), true)
	g.c.linef(1, tagDeclLine)
	g.c.linef(1, tagReadFormat, quote(v.Tag))
	for i, cs := range v.Cases {
		open := elseIfTagFormat
		if i == 0 {
			open = ifTagFormat
		}
		g.c.linef(1, open, quote(cs.Wire))
		h := g.written(g.pl.CaseHook(v, cs))
		if len(cs.Fields) == 0 {
			g.c.linef(depthTwo, assignFormat, outVar, g.makeCall(v.Pkg, h.Name, nil))
			continue
		}
		g.c.base++
		g.readFields(class{variant: v, cs: cs}, v.Pkg, h.Name, h.Members)
		g.c.base--
	}
	g.c.linef(1, elseOpen)
	g.c.linef(depthTwo, unknownCaseFormat, quote(v.Tag))
	g.c.linef(1, closeBrace)
	g.c.linef(1, returnOk)
}

// hookLocals write hookLocalDecls at depth of the source.
func (g *gen) hookLocals(depth int, members []ir.CppHookMember, prefix string) {
	for _, l := range g.hookLocalDecls(members, prefix) {
		g.c.lineAt(depth, l)
	}
}

// movedLocals are a hook's arguments from the locals hookLocals declared.
func movedLocals(members []ir.CppHookMember, prefix string) []string {
	var out []string
	for _, m := range members {
		out = append(out, fmt.Sprintf(moveFormat, prefix+m.Member))
		if m.DefineMember != "" {
			out = append(out, fmt.Sprintf(moveFormat, prefix+m.DefineMember))
		}
	}
	return out
}

// entrySite is where foreignEntryChecks write: the depth, the owner package, the locals' prefix, and the key expressions of fields not read at their wire path (a pairs slot's).
type entrySite struct {
	depth       int
	pkg, prefix string
	wires       map[*ir.Field]string
}

// foreignEntryChecks refuse a key of a resolved getter, a field's or a stored fn's, each lookup cell's included, naming no entry of its owner's keyed list, with the loader text `no entry <key>` at its pointer; a key into a table with an id enum was refused when read (CODEGEN.md §2.8 Refs).
func (g *gen) foreignEntryChecks(at entrySite, members []ir.CppHookMember) {
	for _, m := range members {
		s, name := g.slotOf(m)
		s.list, _ = refSlot(s.t)
		if !m.Resolved || g.idKeyed(s.target()) || emptyLookup(m, s) {
			continue
		}
		find := g.qualifier(at.pkg) + g.ownerAccessor(s.target()) + findCallPrefix
		local := at.prefix + m.Member
		if s.cells == 0 {
			g.keyChecks(at.depth, find, local, at.wire(m, name), s)
			continue
		}
		paths := cellPaths(name, g.domains(m.Fn))
		for i, p := range paths {
			paths[i] = quote(p)
		}
		g.c.linef(at.depth, countForFormat, cellLoopVar, cellLoopVar, s.cells, cellLoopVar)
		g.c.linef(at.depth+1, cellsArrayFormat, strings.Join(paths, listSep))
		g.keyChecks(at.depth+1, find, fmt.Sprintf(indexFormat, local, cellLoopVar), cellsKey, s)
		g.c.linef(at.depth, closeBrace)
	}
}

// wire is the key expression naming member m in a load error: a pairs slot's key, a field's wire path, a stored fn's `$<fn>`.
func (at entrySite) wire(m ir.CppHookMember, name string) string {
	if w, ok := at.wires[m.Field]; ok {
		return w
	}
	if m.Field != nil {
		return quote(wireName(m.Field))
	}
	return quote(dollar + name)
}

// idKeyed reports a ref held as its target table's id enum.
func (g *gen) idKeyed(t ir.TypeRef) bool {
	_, id := g.idEnum(t)
	return id
}

// ownerAccessor is the accessor of the owner's keyed list a ref targets: Get<V>, or that value's @cpp(name:) (CODEGEN.md §3.3, §5.9).
func (g *gen) ownerAccessor(t ir.TypeRef) string { return g.pl.OwnerAccessor(t.Ref) }

// keyChecks refuse each key of slot s held in key at depth that find does not find, wire naming it.
func (g *gen) keyChecks(depth int, find, key, wire string, s slot) {
	text := g.keyText(s.target().Key)
	check := func(depth int, k, at string) {
		g.c.linef(depth, mapKeyCheckFmt, find, k, at, fmt.Sprintf(text, k))
	}
	switch {
	case s.list && s.optional:
		g.c.linef(depth, ifOpenFormat, key)
		g.c.linef(depth+1, forFormat, indexLocal, indexLocal, derefStar+key, indexLocal)
		check(depth+depthTwo, fmt.Sprintf(indexFormat, fmt.Sprintf(derefFormat, key), indexLocal), elemKey(wire, indexLocal))
		g.c.linef(depth+1, closeBrace)
		g.c.linef(depth, closeBrace)
	case s.list:
		g.c.linef(depth, forFormat, indexLocal, indexLocal, key, indexLocal)
		check(depth+1, fmt.Sprintf(indexFormat, key, indexLocal), elemKey(wire, indexLocal))
		g.c.linef(depth, closeBrace)
	case s.optional:
		g.c.linef(depth, mapKeyCheckFmt, key+andSep+find, derefStar+key, wire, fmt.Sprintf(text, derefStar+key))
	default:
		check(depth, key, wire)
	}
}

// decodeEntryID reads a key into a table of another package, held as its owner's id enum: a key naming no entry is `no entry <key>` (CODEGEN.md §2.8, §5.3).
func (g *gen) decodeEntryID(depth int, src, key, id, dst string) {
	k := fmt.Sprintf(entryKeyVarFormat, depth)
	g.c.linef(depth, entryIDOpenFormat, k, src, key)
	g.c.linef(depth+1, entryIDParseFormat, g.pl.EnumHelpers(id).FromWire, k, dst, key)
	g.c.linef(depth, closeBrace)
}

// foreignKey reports a ref of another package's class held as an id enum: its reader parses the key with the enum's FromWire.
func (g *gen) foreignKey(t ir.TypeRef) (string, bool) {
	if g.view == "" || g.view == g.p.Name || t.Kind != types.Ref {
		return "", false
	}
	return g.idEnum(t)
}
