package eval

import "github.com/fantasim/canonlang/internal/value"

// EntryToken is the kept evaluation rec, an entry's record, copies (log-2026-09-29 M4 P12-r).
func (e *Evaluator) EntryToken(rec *value.Record) (any, bool) {
	u := e.memo
	if u == nil || u.tokens == nil {
		return nil, false
	}
	en, ok := u.tokens[rec]
	return en, ok
}

// TokenOwner is the top-level value whose evaluation made the record of token's entry, the one
// whose stages B and C replay it: another holding it read it, and meets it as a cold run does
// (log-2026-09-29 M4 P12-r).
func (e *Evaluator) TokenOwner(token any) (Root, bool) {
	en, ok := token.(*memoEntry)
	if !ok {
		return Root{}, false
	}
	return Root{Pkg: en.owner.pkg, Name: en.owner.name}, true
}

// Completed is the values whose evaluation completed, in order (log-2026-09-29 M4 P12-r).
func (e *Evaluator) Completed() []Root {
	out := make([]Root, len(e.completed))
	for i, st := range e.completed {
		out[i] = st.root
	}
	return out
}

// noteToken keeps en as rec's token when rec shares no node with a value it read.
func (u *memoUse) noteToken(rec *value.Record, en *memoEntry) {
	if rec == nil || len(en.kept.foreign) > 0 {
		return
	}
	if u.tokens == nil {
		u.tokens = map[*value.Record]*memoEntry{}
	}
	u.tokens[rec] = en
}

// readEdge is a value read and a value whose evaluation read it (log-2026-09-29 M4 P18).
type readEdge struct {
	read, by *rootState
}

// ReadBy is the top-level values whose evaluation read root, directly, in first-read order;
// false when the evaluator uses no memo, so no read was noted. A value holds a part of another
// only if its evaluation read it, directly or through values read (log-2026-09-29 M4 P18).
func (e *Evaluator) ReadBy(root Root) ([]Root, bool) {
	if e.memo == nil {
		return nil, false
	}
	st := e.roots[root]
	if st == nil {
		return nil, true
	}
	out := make([]Root, len(st.readBy))
	for i, by := range st.readBy {
		out[i] = by.root
	}
	return out, true
}

// noteRead notes that st is read by the value being evaluated and by the one reader's run is
// charged to (stage B's conversions), when either is a top-level value.
func (e *Evaluator) noteRead(st *rootState, reader *run) {
	u := e.memo
	if u == nil {
		return
	}
	var top *rootState
	if n := len(e.stack); n > 0 {
		top = e.stack[n-1]
		u.readBy(st, top)
	}
	if reader == nil {
		return
	}
	charged := Root{Pkg: reader.charge.pkg, Name: reader.charge.name}
	if top != nil && top.root == charged {
		return
	}
	if by := e.roots[charged]; by != nil {
		u.readBy(st, by)
	}
}

// readBy notes that by's evaluation read st, once.
func (u *memoUse) readBy(st, by *rootState) {
	if st == by || len(st.readBy) > 0 && st.readBy[len(st.readBy)-1] == by {
		return
	}
	edge := readEdge{read: st, by: by}
	if u.edges[edge] {
		return
	}
	if u.edges == nil {
		u.edges = map[readEdge]bool{}
	}
	u.edges[edge] = true
	st.readBy = append(st.readBy, by)
}
