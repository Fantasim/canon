package edit

import (
	"fmt"
	"slices"
)

// trieKey is a segment below a trie position; a root is a segment below noPlace.
type trieKey struct {
	parent int
	seg    Seg
}

// segTrie holds the paths of the nodes by segments, for R12's longest prefix; node is the
// node at each position, noPlace for a mere prefix.
type segTrie struct {
	next map[trieKey]int
	node []int
}

func newSegTrie() segTrie {
	return segTrie{next: map[trieKey]int{}}
}

// add is the position of seg below parent, made when new.
func (t *segTrie) add(parent int, seg Seg) int {
	k := trieKey{parent: parent, seg: seg}
	if id, ok := t.next[k]; ok {
		return id
	}
	id := len(t.node)
	t.node = append(t.node, noPlace)
	t.next[k] = id
	return id
}

// find is the position of seg below parent, noPlace when no node lies there or below.
func (t *segTrie) find(parent int, seg Seg) int {
	if id, ok := t.next[trieKey{parent: parent, seg: seg}]; ok {
		return id
	}
	return noPlace
}

// rootTrieSeg is the trie segment of a let's root: its qualified name.
func rootTrieSeg(r rootRef) Seg {
	return Seg{Kind: SegField, Name: r.qualified()}
}

// homes is, for each candidate, whether it is its identity's node (API.md R11): of the
// candidates of one identity that lie under a node or none, the first whose path stays in its
// source tree (W3), else the first in walk order.
type homes struct {
	ns     *nodeSet
	chosen map[nodeIdent]int // the candidate an identity chose, noPlace for none
}

// is reports candidate i being its identity's node.
func (h *homes) is(i int) bool {
	k := h.ns.cands[i].row.ident
	pick, ok := h.chosen[k]
	if !ok {
		h.chosen[k] = noPlace // in progress: a cycle of identities chooses none
		pick = h.choose(h.ns.byCand[k])
		h.chosen[k] = pick
	}
	return pick == i
}

// choose is the node among one identity's candidates, in walk order; is recurses toward the
// roots only, each candidate's parent lying above it.
func (h *homes) choose(ids []int) int {
	var live []int
	for _, i := range ids {
		if u := h.ns.cands[i].under; u == noPlace || h.is(u) {
			live = append(live, i)
		}
	}
	switch len(live) {
	case 0:
		return noPlace
	case 1:
		return live[0]
	}
	if j := slices.IndexFunc(live, h.stays); j >= 0 {
		return live[j]
	}
	return live[0]
}

// stays reports candidate i's path staying in its root's source tree (API.md W3).
func (h *homes) stays(i int) bool {
	p, err := Parse(h.ns.cands[i].row.ID)
	if err != nil {
		return false
	}
	_, ok := h.ns.s.structural(p)
	return ok
}

// settle keeps one candidate per identity, then orders every node by R14 and indexes them:
// identities, members and trie positions. An identity at two nodes is a compiler bug.
func (ns *nodeSet) settle() error {
	h := &homes{ns: ns, chosen: map[nodeIdent]int{}}
	ns.rows = slices.Grow(ns.rows, len(ns.byCand))
	for i, c := range ns.cands {
		if h.is(i) {
			ns.rows = append(ns.rows, c.row)
		}
	}
	ns.cands, ns.byCand = nil, nil
	slices.SortStableFunc(ns.rows, func(a, b nodeRow) int { return ns.s.nodeOrder(a.GraphNode, b.GraphNode) })
	ns.nodes = make([]GraphNode, len(ns.rows))
	for i, row := range ns.rows {
		ns.nodes[i] = row.GraphNode
		if err := ns.index(i, row); err != nil {
			return err
		}
	}
	ns.rows = nil
	return nil
}

// index makes node i, row, found by its identity or member and its trie position.
func (ns *nodeSet) index(i int, row nodeRow) error {
	switch row.Kind {
	case NodeMember:
		ns.byMember[row.member] = i
	case NodeEntry, NodeElement:
		if j, dup := ns.byIdent[row.ident]; dup {
			return fmt.Errorf("%w: %s and %s are one identity", ErrInternal, ns.nodes[j].ID, row.ID)
		}
		ns.byIdent[row.ident] = i
	default:
	}
	if row.trie != noPlace {
		ns.trie.node[row.trie] = i
	}
	return nil
}
