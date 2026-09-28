package main

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// occurrence is a member as one branch declares it.
type occurrence struct {
	branch   int
	n        *node
	required bool
}

type member struct {
	json string
	occ  []occurrence
}

// members are the members of every branch, in first-seen order, and each branch's order as
// edges between them (index before index).
type members struct {
	list  []*member
	index map[string]int
	edges []edge
}

// edge says member from comes before member to in some branch.
type edge struct{ from, to int }

// flatten fills sd with the union of the branches' members, in an order that keeps every
// branch's own member order (VIEWMODEL.md J2).
func (g *gen) flatten(sd *structDef, branches []*node) error {
	ms := members{index: map[string]int{}}
	for b, br := range branches {
		if err := ms.add(b, br, sd.name); err != nil {
			return err
		}
	}
	order, err := topo(len(ms.list), ms.edges)
	if err != nil {
		return fmt.Errorf(fmtAt, err, sd.name)
	}
	tags := tagValues(branches)
	seen := map[string]bool{}
	for _, i := range order {
		f, err := g.member(sd.name, ms.list[i], len(branches), tags)
		if err != nil {
			return err
		}
		if seen[f.name] {
			return fmt.Errorf(fmtAtMember, errName, sd.name, f.name)
		}
		seen[f.name] = true
		sd.fields = append(sd.fields, f)
	}
	return nil
}

func (ms *members) add(b int, br *node, owner string) error {
	if err := checkObject(br, owner); err != nil {
		return err
	}
	props := br.get(kwProperties)
	req := br.get(kwRequired).texts()
	prev := -1
	for k, name := range props.keys {
		i, ok := ms.index[name]
		if !ok {
			i = len(ms.list)
			ms.index[name] = i
			ms.list = append(ms.list, &member{json: name})
		}
		ms.list[i].occ = append(ms.list[i].occ, occurrence{b, props.vals[k], slices.Contains(req, name)})
		if prev >= 0 {
			ms.edges = append(ms.edges, edge{prev, i})
		}
		prev = i
	}
	return nil
}

// topo orders n items so that every edge's first item comes first, the earliest-seen ready
// item first; a cycle means two branches order two members both ways.
func topo(n int, edges []edge) ([]int, error) {
	indeg := make([]int, n)
	next := make([][]int, n)
	for _, e := range edges {
		next[e.from] = append(next[e.from], e.to)
		indeg[e.to]++
	}
	done := make([]bool, n)
	out := make([]int, 0, n)
	for len(out) < n {
		i := ready(indeg, done)
		if i < 0 {
			return nil, errOrder
		}
		done[i] = true
		out = append(out, i)
		for _, s := range next[i] {
			indeg[s]--
		}
	}
	return out, nil
}

func ready(indeg []int, done []bool) int {
	for i, d := range indeg {
		if d == 0 && !done[i] {
			return i
		}
	}
	return -1
}

// member is the field of one member: the branches' types merged, always present when every
// branch requires it.
func (g *gen) member(owner string, m *member, branches int, tags [][]string) (*field, error) {
	site := owner + fieldSep + m.json
	var out typed
	always := len(m.occ) == branches
	for i, o := range m.occ {
		t, err := g.typeOf(o.n, site)
		if err != nil {
			return nil, err
		}
		if out, err = merge(out, t, i == 0); err != nil {
			return nil, fmt.Errorf(fmtAt, err, site)
		}
		always = always && o.required
	}
	name, err := exported(m.json)
	if err != nil {
		return nil, err
	}
	f := &field{json: m.json, name: name, t: out.t, always: always, zeroOK: out.zeroOK, via: out.via}
	if tags != nil && len(m.occ) < branches {
		for _, o := range m.occ {
			f.kinds = append(f.kinds, tags[o.branch]...)
		}
	}
	return f, nil
}

// merge is the type two branches give one member: equal types, or an integer and a number
// merged into a Number; any other difference is refused.
func merge(a, b typed, first bool) (typed, error) {
	if first {
		return b, nil
	}
	a.zeroOK = a.zeroOK || b.zeroOK
	a.via = cmp.Or(a.via, b.via)
	if a.t.String() == b.t.String() {
		return a, nil
	}
	if numeric(a.t) && numeric(b.t) {
		a.t = &goType{kind: tNumber}
		return a, nil
	}
	return typed{}, fmt.Errorf("%w (%s, %s)", errConflict, a.t, b.t)
}

func numeric(t *goType) bool { return t.kind == tInt || t.kind == tNumber }

// tagValues are the tag values (the kind member's const or enum) of each branch of a union;
// nil when the object is not a union or a branch has no tag.
func tagValues(branches []*node) [][]string {
	if len(branches) <= 1 {
		return nil
	}
	out := make([][]string, 0, len(branches))
	for _, b := range branches {
		tag := b.get(kwProperties).get(tagMember)
		switch {
		case tag.has(kwConst):
			out = append(out, []string{tag.get(kwConst).text})
		case tag.has(kwEnum):
			out = append(out, tag.get(kwEnum).texts())
		default:
			return nil
		}
	}
	return out
}

// comment notes the kinds of the branches holding a field, and the one branch definition a
// shared struct stands for there.
func (f *field) comment() string {
	var notes []string
	if f.kinds != nil {
		notes = append(notes, kindNote+strings.Join(f.kinds, listSep))
	}
	if f.via != "" {
		notes = append(notes, fmt.Sprintf(viaNote, defsPrefix+f.via))
	}
	if notes == nil {
		return ""
	}
	return commentPrefix + strings.Join(notes, noteSep)
}
