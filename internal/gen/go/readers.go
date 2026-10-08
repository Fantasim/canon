package gogen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// readerView is the shape of text/hooks.txt's reader and readerOut: a reader's signature, and the local struct its fields are read into.
type readerView struct {
	Func, Raw, Extra, Type string
	L                      locals
	Members                []readerField
}

// readerField is one member of a reader's local struct: a parameter of the make hook.
type readerField struct{ Name, Type string }

// readers writes this package's reader of each other package's class its loaders read, in first-reach order (CODEGEN.md §2.7, §2.8): it reads the class's wire as this package's own decoders read their classes, then builds the value with the owner's make hook.
func (g *gen) readers() bool {
	read := g.names.Foreign().Read
	for _, class := range read {
		switch x := class.(type) {
		case *ir.Record:
			g.readerFunc(g.bodyOf(x), g.names.RecordHook(x).Name)
		case *ir.Variant:
			g.variantReader(x)
		case *ir.Case:
			v := g.variantOf[x]
			if v == nil {
				g.failf(ErrMalformed, caseNoVariantFormat, x.Name)
				continue
			}
			g.readerFunc(g.caseBody(v, x), g.names.CaseHook(v, x).CaseType)
		case *ir.Dependent:
			g.decodeDependent(x)
		}
	}
	return len(read) > 0
}

// readerFunc writes decode_<gopkg>_<T> of a record or case of another package.
func (g *gen) readerFunc(b *body, hook string) {
	defer g.enter(b.owner)()
	defer g.withRT(b.pkg)()
	g.temps = 0
	g.inlineFolds(b)
	g.exec(tmplReader, readerView{Func: g.decodeFunc(b.key), Raw: g.rawType(), Type: b.goName, L: g.lc})
	g.readInto(b, g.qualify(b.pkg, hook))
	g.ownerChecks(b, g.lc.Dst, g.lc.Out)
	g.printf(returnNil)
}

// readInto reads b's object into a local struct, builds *dst with hook, then refuses a key the owner resolves to no entry (CODEGEN.md §2.8).
func (g *gen) readInto(b *body, hook string) {
	lc := g.lc
	params := g.hookParams(b)
	if len(params) > 0 {
		view := readerView{L: lc}
		for _, p := range params {
			view.Members = append(view.Members, readerField{p.name, p.typ})
		}
		g.exec(tmplReaderOut, view)
	}
	g.openObject(g.bodyKeys(b))
	g.readFields(b)
	args := make([]string, len(params))
	for i, p := range params {
		args[i] = lc.Out + dot + p.name
	}
	g.printf(cellStoreFormat, pointer+lc.Dst, hook+callArgs(args))
}

// ownerChecked reports a class whose reader checks keys the owner's hook resolves into a keyed list.
func ownerChecked(b *body) bool {
	return slices.ContainsFunc(b.slots, func(s *slot) bool { return s.owned && s.Ref.Keyed }) ||
		slices.ContainsFunc(b.finite, func(f *finiteMethod) bool { return f.res.owned && f.res.Ref.Keyed && f.entry != "" })
}

// ownerChecks checks, on recv, the value the hook built from the local struct src, each key the owner resolves into a keyed list (CODEGEN.md §2.8).
func (g *gen) ownerChecks(b *body, recv, src string) {
	for _, s := range b.slots {
		if s.owned && s.Ref.Keyed {
			g.ownerCheck(s, recv, src, g.keyLoc(messageKey(s)))
		}
	}
	for _, f := range b.finite {
		if f.res.owned && f.res.Ref.Keyed && f.entry != "" {
			g.ownerCellCheck(f, recv)
		}
	}
}

// entryCheck is one check of keys of a keyed list the owner's hook resolved: the entry getter call, nil for a key naming no entry, run when present is "" or holds.
type entryCheck struct {
	present, call, keys string
	list, pair          bool // a list of keys; its getter also returns a presence flag
	loc                 location
}

// checkEntries refuses the first key whose entry is nil with the loader text `no entry <key>` (CODEGEN.md §2.8).
func (g *gen) checkEntries(c entryCheck) {
	if c.present != "" {
		g.printf(ifOpenFormat, c.present)
	}
	switch {
	case !c.list:
		g.printf(ownerCheckFormat, "", c.call, g.errAt(c.loc, noEntryText, c.keys))
	default:
		v, j := g.temp(tempValue), g.temp(tempIndex)
		if c.pair {
			g.printf(entriesPairFormat, v, c.call)
		} else {
			g.printf(defineFormat, v, c.call)
		}
		g.printf(ownerListCheckFormat, j, c.keys, v, g.errAt(c.loc.index(j), noEntryText, c.keys+atCall+j+rparen))
	}
	if c.present != "" {
		g.printf(closeBrace)
	}
}

// ownerCheck checks a field the owner's hook resolves into its keyed list: recv's resolved getter, the keys read into src.
func (g *gen) ownerCheck(s *slot, recv, src string, loc location) {
	c := entryCheck{list: s.List, pair: s.List && s.Optional, loc: loc}
	c.call, c.keys = recv+dot+s.Getter+callSuffix, src+dot+s.KeyStore
	if s.Optional {
		c.present = src + dot + s.OKStore
	}
	g.checkEntries(c)
}

// ownerCellCheck checks each cell of a lookup the owner's hook resolves into its keyed list: recv's entry getter called on the cell's domain values (CODEGEN.md §5.10; log-2026-10-06 "U2 review FAIL" 2).
func (g *gen) ownerCellCheck(f *finiteMethod, recv string) {
	lc := g.lc
	c := entryCheck{list: f.res.List, pair: f.res.List && f.res.Optional, loc: g.keyLoc(dollar + f.fn.Name)}
	c.keys = lc.Out + dot + f.store
	if f.split {
		c.present = lc.Out + dot + f.res.OKStore
	}
	args := make([]string, len(f.fn.Params))
	for n, p := range f.fn.Params {
		i, k := g.temp(tempIndex), g.temp(tempKey)
		g.printf(domainLoopFormat, i, k, strings.Join(g.domainKeys(f.fn, n), listSep))
		args[n] = g.domainValue(p.Type, i)
		c.keys, c.loc = c.keys+lbracket+i+rbracket, c.loc.dot().arg(k)
		if c.present != "" {
			c.present += lbracket + i + rbracket
		}
	}
	c.call = recv + dot + f.entry + lparen + strings.Join(args, listSep) + rparen
	g.checkEntries(c)
	g.body.WriteString(strings.Repeat(closeBrace, len(f.fn.Params)))
}

// domainValue is the value at index i of a finite parameter's domain, in domain order (CODEGEN.md §5.10): false then true, or an enum's members.
func (g *gen) domainValue(t ir.TypeRef, i string) string {
	if t.Kind == types.Ref { // an id enum's value is its entry's position (CODEGEN.md §5.3)
		return g.keyType(t) + lparen + i + rparen
	}
	if t.Kind == types.Bool {
		return fmt.Sprintf(domainValueFormat, goBool, strconv.FormatBool(false)+listSep+strconv.FormatBool(true), i)
	}
	e, ok := t.Named.(*ir.Enum)
	if !ok {
		g.failf(ErrMalformed, "a lookup parameter of %s that is no Bool or enum", g.at)
		return nilLit
	}
	members := make([]string, len(e.Members))
	for n := range e.Members {
		members[n] = g.memberLit(t, n)
	}
	return fmt.Sprintf(domainValueFormat, g.typeName(e), strings.Join(members, listSep), i)
}

// variantReader writes decode_<gopkg>_<V> of another package's variant: the tag, then the case's fields read from the same object into the case's make hook.
func (g *gen) variantReader(v *ir.Variant) {
	defer g.enter(v.QName())()
	defer g.withRT(v.Pkg)()
	g.temps = 0
	lc := g.lc
	g.exec(tmplReader, readerView{Func: g.decodeFunc(v), Raw: g.rawType(), Type: g.typeName(v), L: lc})
	g.openObject([]string{v.Tag})
	loc := g.keyLoc(v.Tag)
	r := g.temp(tempRaw)
	g.printf(needFormat, r, lc.Err, g.helper(helperNeed), lc.Name, lc.Path, lc.Obj, strconv.Quote(v.Tag))
	g.readPlainInto(lc.Tag, goString, r, loc)
	g.printf(switchTagFormat, lc.Tag)
	for _, c := range v.Cases {
		g.printf(caseFormat, strconv.Quote(c.Wire))
		h := g.qualify(v.Pkg, g.names.CaseHook(v, c).Name)
		if len(c.Fields) == 0 {
			g.printf(cellStoreFormat, pointer+lc.Dst, h+lparen+rparen)
			continue
		}
		b := g.caseBody(v, c)
		g.inlineFolds(b)
		g.readInto(b, h)
		if ownerChecked(b) { // on the case the hook built, never the variant (log-2026-10-06 "gen/go re-verify FAIL")
			x := g.temp(tempElem)
			g.printf(entriesPairFormat, x, lc.Dst+dot+g.names.AsName(c)+callSuffix)
			g.ownerChecks(b, x, lc.Out)
		}
	}
	g.printf(unknownCaseFormat, g.errAt(loc, unknownCaseText, lc.Tag))
	g.printf(returnNil)
}
