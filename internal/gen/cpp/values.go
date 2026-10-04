package cppgen

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

var (
	//go:embed text/translated_doc.txt
	translatedDocText string
	//go:embed text/snapshot_doc.txt
	snapshotDocText string
	//go:embed text/snapshot_load_doc.txt
	snapshotLoadDocText string
	//go:embed text/store.txt
	storeText string
	//go:embed text/conformance_decl.txt
	conformanceDeclText string
	//go:embed text/move_only.txt
	moveOnlyText string
)

// detailDecls: package fn prototypes when a pure fn calls one (log-2026-09-24), then §2.7 step 4.
func (g *gen) detailDecls() {
	if g.callsPackageFn() {
		for _, fn := range g.p.Fns {
			leave := g.enter(fn.Name)
			switch {
			case fn.Kind == ir.FnTranslated:
				g.h.printf(protoFormat, g.pureSignature(g.pl.FnName(fn), fn))
			case g.pl.Constexpr(fn):
				r := g.fnProto(fn)
				g.h.printf(declFormat, fmt.Sprintf(signatureFormat, r.typ, r.name, r.params))
			default:
				r := g.fnReader(fn)
				g.h.printf(declFormat, fmt.Sprintf(signatureFormat, r.typ, r.name, r.params))
			}
			leave()
		}
		g.h.blank()
	}
	g.h.line(detailOpen)
	g.h.printf(accessDeclFormat, g.pl.AccessName())
	for _, c := range g.declared() {
		if g.baked() {
			break // nothing is decoded (CODEGEN.md §2.2)
		}
		if c.dependent != nil {
			g.dependentDecodeDecl(c.dependent)
		} else {
			g.h.printf(decodeDeclFormat, g.className(c))
		}
	}
	for _, m := range g.methods {
		leave := g.enter(m.class.canonName() + qnameSep + m.fn.Name)
		if m.fn.File == "" {
			g.fail(fmt.Errorf("%w: translated fn %s without its source file", ErrMalformed, g.at))
		}
		g.h.blank()
		g.h.printf(translatedDocText, g.p.Dir+pathSep+m.fn.File, m.class.canonName(), m.fn.Name)
		g.pureFn(g.pl.PureName(g.className(m.class), m.fn), m.fn)
		leave()
	}
	g.inputSlots()
	g.h.line(detailClose)
	g.h.blank()
}

// packageFns are the package-level export fns in declaration order: a translated one public and pure at once, a baked stored one constexpr in the header or declared here and defined in the source (CODEGEN.md §5.10, decision 293).
func (g *gen) packageFns() {
	for _, fn := range g.p.Fns {
		leave := g.enter(fn.Name)
		switch {
		case fn.Kind == ir.FnTranslated:
			g.doc(0, fn.Doc)
			g.pureFn(g.pl.FnName(fn), fn)
		case g.pl.Constexpr(fn):
			g.constexprFn(fn)
		default:
			g.storedFnDecl(fn)
		}
		g.h.blank()
		leave()
	}
}

// containers are the classes of table and keyed-list values (CODEGEN.md §5.9, T8).
func (g *gen) containers() {
	for _, v := range g.values {
		if !ir.IsContainer(v) {
			continue
		}
		leave := g.enter(v.Name)
		g.container(v)
		leave()
	}
}

// containerSpec is what a container's text needs: element, key, @stable fields, wireKey (empty for a table's fixed row id, else the keyed-by field's full wire path) and render, the duplicate-id token renderer (WIRE.md §5.7, §7.2, §7.3).
type containerSpec struct {
	name, elem, key, keyName, wireKey, render string
	stable                                    []*ir.Field
}

func (g *gen) containerSpec(v *ir.Value) containerSpec {
	rec, ok := v.Type.Elem.Named.(*ir.Record)
	if !ok {
		g.fail(fmt.Errorf("%w: value %s of no record", ErrMalformed, v.Name))
		return containerSpec{}
	}
	s := containerSpec{name: g.pl.ContainerName(v), elem: g.typeName(rec), key: cppString, keyName: idKeyName}
	if v.Type.KeyedBy != nil {
		kf := g.keyField(v.Type)
		if kf == nil {
			return s
		}
		s.key, s.keyName = g.storage(kf.Type), kf.Name
		s.wireKey, s.render = strings.Join(kf.WirePath, qnameSep), g.keyRenderer(kf.Type)
	}
	for _, f := range rec.Fields {
		if f.Stable && f.Optional {
			// check's stableFields (E6003) already refuses an optional @stable field: unreachable.
			g.fail(fmt.Errorf("%w: optional @stable field %s", ErrMalformed, f.Name))
		} else if f.Stable {
			s.stable = append(s.stable, f)
		}
	}
	return s
}

// keyRenderer is the C++ lambda CheckUnique renders a duplicate key's token with: generic KeyToken for String, an integer or a ref, else an enum's wire string or, under @json(codes), its code (WIRE.md §5.3, §7.2, §7.3).
func (g *gen) keyRenderer(t ir.TypeRef) string {
	for t.Kind == types.Ref && t.Key != nil {
		t = *t.Key
	}
	e, ok := t.Named.(*ir.Enum)
	if t.Kind != types.Enum || !ok {
		return genericKeyRender
	}
	if e.JSONCodes {
		return fmt.Sprintf(codeKeyRenderFormat, g.typeName(t.Named), g.storage(*e.Codes))
	}
	return fmt.Sprintf(wireKeyRenderFormat, g.typeName(t.Named))
}

func (g *gen) container(v *ir.Value) {
	s := g.containerSpec(v)
	sc := newScope(s.name)
	for _, n := range containerNames {
		g.fail(sc.add(n, s.name))
	}
	g.doc(0, v.Doc)
	g.h.printf(classOpenFormat, s.name)
	g.h.printf(moveOnlyText, s.name)
	if !v.Reload && !g.baked() {
		g.fail(sc.add(ir.CppLoad, v.Name))
		g.h.linef(1, loadDeclFormat, s.name)
	}
	g.h.linef(1, lenFormat)
	g.h.linef(1, atFormat, s.elem)
	g.h.linef(1, allFormat, s.elem)
	if rec, ok := v.Type.Elem.Named.(*ir.Record); ok && g.baked() && v.Type.Kind == types.Table {
		g.fail(sc.add(ir.GoGet, v.Name))
		g.h.linef(1, getFormat, s.elem, g.pl.IDName(rec))
	}
	g.h.linef(1, findDocFormat, s.keyName)
	g.h.linef(1, findFormat, s.elem, lookupType(s.key), verbatim(s.keyName))
	for _, f := range s.stable {
		g.findBy(sc, s, f)
	}
	g.h.blank()
	g.h.line(privateLabel)
	g.h.linef(1, friendAccessFormat, g.pl.AccessName())
	g.h.blank()
	g.h.linef(1, rowsMemberFormat, s.key, s.elem)
	for _, f := range s.stable {
		fb := g.pl.FindBy(f)
		g.h.linef(1, vectorDeclFormat, g.storage(f.Type), fb.Keys)
		g.h.linef(1, vectorDeclFormat, cppUint32, fb.Index)
	}
	g.h.line(closeClass)
	g.h.blank()
}

// findBy is FindBy<F>: escaped parameter, a local apart from it, names declared (§3.4, §5.9).
func (g *gen) findBy(sc *scope, s containerSpec, f *ir.Field) {
	fb := g.pl.FindBy(f)
	param, local := verbatim(f.Name), indexLocal
	for local == param {
		local += underscore
	}
	for _, n := range []string{fb.Func, fb.Keys, fb.Index} {
		g.fail(sc.add(n, f.Name))
	}
	g.h.linef(1, findByOpenFormat, s.elem, fb.Func, lookupType(g.storage(f.Type)), param)
	g.h.linef(depthTwo, findSortedFormat, local, fb.Keys, fb.Index, param)
	g.h.linef(depthTwo, findResultFormat, local)
	g.h.linef(1, closeBrace)
}

// lookupType is what a lookup takes: std::string_view for a string key (canon::LookupKey).
func lookupType(key string) string {
	if key == cppString {
		return cppStringView
	}
	return key
}

// snapshot is <P>Snapshot and <P>Store over every @reload value (CODEGEN.md §5.11, T4–T6, T9).
func (g *gen) snapshot() {
	if g.reloads() == 0 {
		return
	}
	snap, store := g.pl.SnapshotName(), g.pl.StoreName()
	var files []string
	for _, v := range g.values {
		if v.Reload {
			files = append(files, dataFile(v))
		}
	}
	g.h.printf(snapshotDocText, g.p.Name)
	g.h.printf(classOpenFormat, snap)
	g.h.printf(moveOnlyText, snap)
	g.h.printf(snapshotLoadDocText, strings.Join(files, listSep))
	g.h.linef(1, snapshotLoadFormat, snap)
	sc := newScope(snap)
	var members []string
	for _, v := range g.values {
		if !v.Reload {
			continue
		}
		m, err := g.member(v.Name)
		g.fail(err)
		name := g.pl.SnapshotGetter(v)
		g.h.blank()
		g.doc(1, v.Doc)
		g.getter(sc, name, fmt.Sprintf(getterFormat, fmt.Sprintf(constRefFormat, g.valueClass(v)), name, "", fmt.Sprintf(returnFormat, m)), m, v.Name)
		members = append(members, fmt.Sprintf(memberFormat, g.valueClass(v), m, ""))
	}
	g.h.blank()
	g.h.line(privateLabel)
	g.h.linef(1, friendAccessFormat, g.pl.AccessName())
	g.h.blank()
	for _, m := range members {
		g.h.lineAt(1, m)
	}
	g.h.line(closeClass)
	g.h.blank()
	g.h.printf(storeText, g.p.Name, store, snap, g.pl.AccessName())
}

// valueClass is a value's container class, or its record type (CODEGEN.md §5.9).
func (g *gen) valueClass(v *ir.Value) string {
	if v.Type.Kind == types.Record {
		return g.typeName(v.Type.Named)
	}
	return g.pl.ContainerName(v)
}

// conformanceDecl declares Run<P>Conformance with T10 (CODEGEN.md §2.7 step 6).
func (g *gen) conformanceDecl() {
	if g.translated() {
		g.h.printf(conformanceDeclText, g.last, g.pl.RunConformanceName())
	}
}
