package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// typeOf is the Go type of a schema node met at site (struct.member), which names the struct
// of an inline object.
func (g *gen) typeOf(n *node, site string) (typed, error) {
	if err := checkKeys(n, knownKeywords, site); err != nil {
		return typed{}, err
	}
	switch {
	case n.has(kwRef):
		return g.refType(n.get(kwRef).text, site)
	case n.has(kwProperties):
		return g.inlineStruct(n, site)
	case n.has(kwConst):
		return constType(n.get(kwConst), site)
	case n.has(kwEnum):
		return enumType(n.get(kwEnum), site)
	case n.has(kwOneOf):
		return g.oneOfType(n, site)
	case !n.has(kwType):
		return typed{t: &goType{kind: tRaw}}, nil // no constraint: any JSON value (vmValue)
	}
	return g.jsonType(n, site)
}

// refType is the type of a $ref: a hand-written type, a scalar, or the struct of the
// definition (of the union it is a branch of, when it is one).
func (g *gen) refType(ref, site string) (typed, error) {
	name, ok := strings.CutPrefix(ref, defsPrefix)
	if !ok || !g.defs.has(name) {
		return typed{}, fmt.Errorf(fmtAt, errRef, ref)
	}
	if k, ok := specialDefs[name]; ok {
		return typed{t: &goType{kind: k}}, nil
	}
	via := ""
	if o, ok := g.owner[name]; ok {
		name, via = o, name
	}
	def := g.defs.get(name)
	if !g.objectish(def) {
		return g.typeOf(def, site)
	}
	sd, err := g.defStruct(name, def)
	out := structType(sd)
	out.via = via
	return out, err
}

func structType(sd *structDef) typed { return typed{t: &goType{kind: tStruct, def: sd}} }

func (g *gen) defStruct(name string, def *node) (*structDef, error) {
	if sd := g.byDef[name]; sd != nil {
		return sd, nil
	}
	goName, err := exported(name)
	if err != nil {
		return nil, err
	}
	branches, err := g.branches(def, name)
	if err != nil {
		return nil, err
	}
	doc := fmt.Sprintf(docDef, defsPrefix+name)
	if len(branches) > 1 {
		doc = fmt.Sprintf(docUnion, defsPrefix+name, len(branches))
	}
	sd, err := g.newStruct(goName, doc)
	if err != nil {
		return nil, err
	}
	g.byDef[name] = sd
	return sd, g.flatten(sd, branches)
}

// inlineStruct is the struct of an object (or oneOf of objects) written in place; the same
// schema met twice is one struct.
func (g *gen) inlineStruct(n *node, site string) (typed, error) {
	name, err := g.inlineName(site)
	if err != nil {
		return typed{}, err
	}
	key := n.canonical()
	if sd := g.byShape[key]; sd != nil {
		return structType(sd), nil
	}
	sd, err := g.newStruct(name, fmt.Sprintf(docInline, site))
	if err != nil {
		return typed{}, err
	}
	g.byShape[key] = sd
	branches, err := g.branches(n, site)
	if err != nil {
		return typed{}, err
	}
	return structType(sd), g.flatten(sd, branches)
}

func (g *gen) inlineName(site string) (string, error) {
	g.used[site] = true
	if i := slices.IndexFunc(g.names, func(o nameOverride) bool { return o.site == site }); i >= 0 {
		return g.names[i].name, nil
	}
	parent, member, _ := strings.Cut(site, fieldSep)
	m, err := exported(member)
	return parent + m, err
}

// oneOfType is a union: of objects, one struct; of scalars, the scalar that holds them all.
func (g *gen) oneOfType(n *node, site string) (typed, error) {
	if g.objectish(n) {
		return g.inlineStruct(n, site)
	}
	var out typed
	kinds := map[typeKind]bool{}
	decimal := false
	for _, b := range n.get(kwOneOf).vals {
		t, err := g.typeOf(b, site)
		if err != nil {
			return typed{}, err
		}
		kinds[t.t.kind] = true
		out.zeroOK = out.zeroOK || t.zeroOK
		decimal = decimal || b.get(kwPattern).textOr() == decimalPattern
	}
	numeric := kinds[tInt] || kinds[tNumber]
	switch {
	case len(kinds) == 1 && kinds[tString]:
		out.t = &goType{kind: tString}
	case len(kinds) == scalarKinds && kinds[tString] && numeric && decimal:
		out.t = &goType{kind: tNumber} // VIEWMODEL.md J10: a number or an integer's decimal string
	case len(kinds) == scalarKinds && kinds[tString] && kinds[tInt]:
		out.t = &goType{kind: tScalar}
	default:
		return typed{}, fmt.Errorf(fmtAt, errConflict, site)
	}
	return out, nil
}

func (g *gen) jsonType(n *node, site string) (typed, error) {
	name := n.get(kwType).text
	if k, ok := jsonTypes[name]; ok {
		return scalarType(k, n)
	}
	var elem *node
	kind := tSlice
	switch name {
	case jsonArray:
		elem = n.get(kwItems)
	case jsonObject:
		elem, kind = n.get(kwAdditional), tMap
	default:
		return typed{}, fmt.Errorf(fmtAtMember, errSchema, site, name)
	}
	t, err := g.typeOf(elem, site)
	if err != nil {
		return typed{}, err
	}
	return typed{t: &goType{kind: kind, elem: t.t}, via: t.via}, nil
}

func scalarType(k typeKind, n *node) (typed, error) {
	out := typed{t: &goType{kind: k}, zeroOK: k == tBool || k == tInt && intZeroOK(n)}
	if k != tString {
		return out, nil
	}
	ok, err := stringZeroOK(n)
	out.zeroOK = ok
	return out, err
}

func constType(c *node, site string) (typed, error) {
	k, ok := scalarOf[c.kind]
	if !ok || !integral(c) {
		return typed{}, fmt.Errorf(fmtAt, errSchema, site)
	}
	return typed{t: &goType{kind: k}, zeroOK: c.text == zeroTexts[c.kind]}, nil
}

// enumType is a string or an integer: an enum mixing JSON kinds is refused.
func enumType(e *node, site string) (typed, error) {
	if e.kind != kindArray || len(e.vals) == 0 {
		return typed{}, fmt.Errorf(fmtAt, errSchema, site)
	}
	first := e.vals[0].kind
	if slices.ContainsFunc(e.vals, func(v *node) bool { return v.kind != first }) {
		return typed{}, fmt.Errorf(fmtAt, errConflict, site)
	}
	k, ok := scalarOf[first]
	if !ok || slices.ContainsFunc(e.vals, func(v *node) bool { return !integral(v) }) {
		return typed{}, fmt.Errorf(fmtAt, errSchema, site)
	}
	zero := slices.ContainsFunc(e.vals, func(v *node) bool { return v.text == zeroTexts[first] })
	return typed{t: &goType{kind: k}, zeroOK: zero}, nil
}

// integral reports a const or enum value vmgen can type: anything but a number that is not an
// integer (it would be generated as int).
func integral(v *node) bool {
	if v.kind != kindNumber {
		return true
	}
	_, err := strconv.ParseInt(v.text, 10, 64)
	return err == nil
}
