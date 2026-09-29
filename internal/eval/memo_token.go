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
