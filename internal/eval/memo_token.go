package eval

import "github.com/fantasim/canonlang/internal/value"

// EntryToken stands for the evaluation of the entry of record rec, which the memo replayed or
// recorded and which shares no node with a value it read: the records of one token are copies
// of one graph, marks included. False: none.
func (e *Evaluator) EntryToken(rec *value.Record) (any, bool) {
	u := e.memo
	if u == nil || u.tokens == nil {
		return nil, false
	}
	en, ok := u.tokens[rec]
	return en, ok
}

// Completed is the top-level values whose evaluation completed, in the order it did: a value can
// hold a part of another only when it completed after it.
func (e *Evaluator) Completed() []Root {
	out := make([]Root, len(e.completed))
	for i, st := range e.completed {
		out[i] = st.root
	}
	return out
}

// noteToken keeps en as the token of rec, its record, when no node of it is a value's it read.
func (u *memoUse) noteToken(rec *value.Record, en *memoEntry) {
	if rec == nil || len(en.kept.foreign) > 0 {
		return
	}
	if u.tokens == nil {
		u.tokens = map[*value.Record]*memoEntry{}
	}
	u.tokens[rec] = en
}
