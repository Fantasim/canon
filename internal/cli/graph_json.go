package cli

import (
	canon "github.com/fantasim/canonlang/api"
)

// graphNodeLine is one node's JSON line (CLI.md §3.18); the key order is the struct's.
type graphNodeLine struct {
	Node struct {
		ID      string         `json:"id"`
		Kind    canon.NodeKind `json:"kind"`
		Package string         `json:"package"`
		Type    string         `json:"type"`
		Retired bool           `json:"retired,omitempty"`
		File    string         `json:"file,omitempty"`
		Line    int            `json:"line,omitempty"`
		Col     int            `json:"col,omitempty"`
	} `json:"node"`
}

// graphEdgeLine is one edge's JSON line (CLI.md §3.18).
type graphEdgeLine struct {
	Edge struct {
		From     string        `json:"from"`
		Path     string        `json:"path"`
		Pattern  string        `json:"pattern"`
		To       string        `json:"to"`
		Kind     canon.RefKind `json:"kind"`
		Computed bool          `json:"computed,omitempty"`
		File     string        `json:"file,omitempty"`
		Line     int           `json:"line,omitempty"`
		Col      int           `json:"col,omitempty"`
	} `json:"edge"`
}

// graphSummary is the JSON summary line of graph.
type graphSummary struct {
	Summary struct {
		Nodes  int `json:"nodes"`
		Edges  int `json:"edges"`
		Groups int `json:"groups"`
	} `json:"summary"`
}

// graphJSON streams one line per item as the API yields it, then the summary. The groups are
// counted the way the text form groups them, from the node types and the edges seen.
func (inv *invocation) graphJSON(p *canon.Project) error {
	types := map[string]string{}
	groups := map[graphKey]bool{}
	var s graphSummary
	err := p.Graph(inv.ctx, func(it canon.GraphItem) error {
		if it.Node != nil {
			s.Summary.Nodes++
			types[it.Node.ID] = it.Node.Type
			return inv.writeJSONLine(nodeLine(it.Node))
		}
		s.Summary.Edges++
		groups[edgeKey(it.Edge, types)] = true
		return inv.writeJSONLine(edgeLine(it.Edge))
	})
	if err != nil {
		return err
	}
	s.Summary.Groups = len(groups)
	return inv.writeJSONLine(s)
}

// nodeLine is n as its JSON line.
func nodeLine(n *canon.GraphNode) graphNodeLine {
	var l graphNodeLine
	l.Node.ID, l.Node.Kind, l.Node.Package, l.Node.Type, l.Node.Retired = n.ID, n.Kind, n.Package, n.Type, n.Retired
	l.Node.File, l.Node.Line, l.Node.Col = n.File, n.Line, n.Col
	return l
}

// edgeLine is e as its JSON line.
func edgeLine(e *canon.GraphEdge) graphEdgeLine {
	var l graphEdgeLine
	l.Edge.From, l.Edge.Path, l.Edge.Pattern, l.Edge.To = e.From, e.Path, e.Pattern, e.To
	l.Edge.Kind, l.Edge.Computed = e.Kind, e.Computed
	l.Edge.File, l.Edge.Line, l.Edge.Col = e.File, e.Line, e.Col
	return l
}
