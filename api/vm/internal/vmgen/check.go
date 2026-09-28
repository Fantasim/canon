package main

import (
	"fmt"
	"slices"
	"strconv"
)

// checkKeys refuses a node that is not a schema object, or that uses a keyword outside allowed.
func checkKeys(n *node, allowed map[string]bool, site string) error {
	if n == nil || n.kind != kindObject {
		return fmt.Errorf(fmtAt, errSchema, site)
	}
	for _, k := range n.keys {
		if !allowed[k] {
			return fmt.Errorf(fmtAtMember, errKeyword, site, k)
		}
	}
	return nil
}

// checkObject refuses an object schema vmgen cannot carry faithfully: an unknown keyword, one
// not closed (type object, additionalProperties false: VIEWMODEL.md V2), or a validation
// keyword that is unknown or names an undeclared member.
func checkObject(br *node, owner string) error {
	if err := checkKeys(br, knownKeywords, owner); err != nil {
		return err
	}
	closed := br.get(kwAdditional)
	if br.get(kwType).textOr() != jsonObject || closed == nil || closed.kind != kindBool ||
		closed.text != strconv.FormatBool(false) || !br.has(kwProperties) {
		return fmt.Errorf(fmtAt, errObject, owner)
	}
	return checkNames(br, owner)
}

// checkNames refuses a required, dependentRequired or validation sub-schema naming a member
// the object does not declare.
func checkNames(br *node, owner string) error {
	named := br.get(kwRequired).texts()
	dependent := br.get(kwDependent)
	for i, k := range dependent.keysOr() {
		named = append(append(named, k), dependent.vals[i].texts()...)
	}
	for _, kw := range []string{kwOneOf, kwAllOf} {
		for _, v := range br.get(kw).valsOr() {
			more, err := validationNames(v, owner)
			if err != nil {
				return err
			}
			named = append(named, more...)
		}
	}
	props := br.get(kwProperties)
	for _, name := range named {
		if !props.has(name) {
			return fmt.Errorf(fmtAtMember, errRequired, owner, name)
		}
	}
	return nil
}

// validationNames lists the members a validation-only sub-schema names, refusing a keyword it
// does not know there.
func validationNames(n *node, owner string) ([]string, error) {
	if err := checkKeys(n, validationKeywords, owner); err != nil {
		return nil, err
	}
	named := append(n.get(kwRequired).texts(), n.get(kwProperties).keysOr()...)
	for _, v := range n.get(kwProperties).valsOr() {
		if err := checkKeys(v, conditionKeywords, owner); err != nil {
			return nil, err
		}
	}
	subs := slices.Concat(n.get(kwOneOf).valsOr(), n.get(kwAllOf).valsOr())
	for _, kw := range []string{kwIf, kwThen} {
		if v := n.get(kw); v != nil {
			subs = append(subs, v)
		}
	}
	for _, v := range subs {
		more, err := validationNames(v, owner)
		if err != nil {
			return nil, err
		}
		named = append(named, more...)
	}
	return named, nil
}

// checkDefs refuses a definition that is only a $ref (an alias) and one that reaches itself
// with no object between (a Go type cannot express it).
func (g *gen) checkDefs() error {
	for i, name := range g.defKeys() {
		def := g.defs.vals[i]
		if def.has(kwRef) {
			return fmt.Errorf(fmtAt, errAlias, name)
		}
		if err := g.acyclic(def, map[*node]bool{}); err != nil {
			return fmt.Errorf(fmtAt, err, name)
		}
	}
	return nil
}

func (g *gen) acyclic(n *node, path map[*node]bool) error {
	if n == nil || n.has(kwProperties) {
		return nil
	}
	if path[n] {
		return errCycle
	}
	path[n] = true
	defer delete(path, n)
	next := slices.Concat(n.get(kwOneOf).valsOr(), []*node{n.get(kwItems), n.get(kwAdditional)})
	if ref := n.get(kwRef); ref != nil {
		next = append(next, g.resolve(ref.text))
	}
	for _, v := range next {
		if v == nil || v.kind != kindObject {
			continue
		}
		if err := g.acyclic(v, path); err != nil {
			return err
		}
	}
	return nil
}
