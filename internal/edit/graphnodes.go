package edit

import (
	"context"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// nodeIdent is an entry's identity as Refs compares it (refs.go isValue): collection, owner, key.
type nodeIdent struct {
	coll  *types.Collection
	owner *value.Record
	key   value.Key
}

// nodeMember is an enum member as Refs compares it.
type nodeMember struct {
	enum  *types.EnumType
	index int
}

// nodeRow is a node and what finds it again: its trie position (noPlace for a member) and its
// identity (the zero key for a root or a member).
type nodeRow struct {
	GraphNode
	trie   int
	ident  nodeIdent
	member nodeMember
}

// candidate is an entry or element met by the node walk where its identity may be at home,
// and the candidate it lies under (noPlace for none): one per identity becomes its node.
type candidate struct {
	row   nodeRow
	rec   *value.Record
	under int
}

// nodeSet is the graph's nodes in R14 order and how a reference finds its target among them.
type nodeSet struct {
	s        *Snapshot
	rows     []nodeRow
	cands    []candidate
	byCand   map[nodeIdent][]int
	trie     segTrie
	nodes    []GraphNode
	byIdent  map[nodeIdent]int
	byMember map[nodeMember]int
	failed   error
	failedAt rootRef
}

// graphNodes walks every declaration of every package: its lets, their entries and elements
// at home, and the members of its enums (API.md R11, R15).
func (s *Snapshot) graphNodes(ctx context.Context) (*nodeSet, error) {
	ns := &nodeSet{
		s: s, byCand: map[nodeIdent][]int{}, trie: newSegTrie(),
		byIdent: map[nodeIdent]int{}, byMember: map[nodeMember]int{},
	}
	for _, pkg := range s.pkgs {
		for _, obj := range pkg.Decls {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			ns.decl(rootRef{pkg: pkg, obj: obj})
		}
	}
	return ns, ns.settle()
}

// decl adds a let and what it holds, or an enum's members.
func (ns *nodeSet) decl(r rootRef) {
	switch r.obj.Kind() {
	case check.ObjLet:
		ns.let(r)
	case check.ObjTypeName:
		if e, ok := rootOf(r.pkg, r.obj, r.obj.Name()); ok && e.enum != nil {
			ns.members(e)
		}
	default:
	}
}

// let adds a root, then the entries and elements its value holds (API.md R11); a let typed
// Error, or one whose missing value could hold a node or a reference, fails the graph (R15).
func (ns *nodeSet) let(r rootRef) {
	pos := ns.trie.add(noPlace, rootTrieSeg(r))
	ns.rows = append(ns.rows, nodeRow{GraphNode: GraphNode{
		ID: r.qualified(), Kind: NodeRoot, Package: r.pkg.Path, Type: r.obj.Type(), Span: nameSpan(r, source.Span{}),
	}, trie: pos})
	t := r.obj.Type()
	v, err := ns.s.force(r)
	if t != nil && t.Base().Kind() == types.Error || err != nil && typeHolds(t, graphLeaf, map[types.Type]bool{}) {
		ns.fail(r, err)
	}
	if err != nil {
		return
	}
	nw := &nodeWalk{ns: ns, r: r, vals: []value.Value{v}, pos: []int{pos}, near: []int{noPlace}}
	nw.below(v, t)
}

// members adds every member of a declared enum (API.md R11, P7a), named as Resolve names it.
func (ns *nodeSet) members(r rootRef) {
	d, _ := r.obj.Decl().(*syntax.EnumDecl)
	for i, m := range r.enum.Members {
		row := nodeRow{GraphNode: GraphNode{
			ID:   Path{Package: r.pkg.Path, Root: r.obj.Name(), Segs: []Seg{{Kind: SegField, Name: m.Name}}}.String(),
			Kind: NodeMember, Package: r.pkg.Path, Type: r.enum, Retired: m.Retired,
		}, trie: noPlace, member: nodeMember{r.enum, i}}
		if f := r.obj.File(); d != nil && f != nil && i < len(d.Members) {
			row.Span = f.Span(d.Members[i].Name)
		}
		ns.rows = append(ns.rows, row)
	}
}

// fail keeps the let R15 names: the first in (package, name) byte order.
func (ns *nodeSet) fail(r rootRef, err error) {
	if ns.failed != nil && !rootBefore(r, ns.failedAt) {
		return
	}
	if err == nil {
		err = ns.s.noValue(r)
	}
	ns.failed, ns.failedAt = err, r
}

func rootBefore(a, b rootRef) bool {
	if a.pkg.Path != b.pkg.Path {
		return a.pkg.Path < b.pkg.Path
	}
	return a.obj.Name() < b.obj.Name()
}

// graphLeaf decides the types R15 counts as able to hold a reference or a node by themselves.
func graphLeaf(b types.Type) (holds, decided bool) {
	switch x := b.(type) {
	case *types.TableType, *types.RefType, *types.EnumType:
		return true, true
	case *types.ListType:
		if x.KeyedBy != nil {
			return true, true
		}
	}
	switch b.Kind() {
	case types.TypeApp, types.DepUnion, types.Any:
		return true, true
	default:
	}
	return false, false
}
