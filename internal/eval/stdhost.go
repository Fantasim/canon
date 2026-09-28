package eval

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// stdHost is how the standard library reaches the run that calls it (std.Host).
type stdHost struct {
	r *run
}

func (r *run) host() *stdHost {
	if r.h == nil {
		r.h = &stdHost{r: r}
	}
	return r.h
}

// std is a built-in's result, nil once it aborted the root; TS-mode code checks it (CONFORMANCE.md §4).
func (r *run) std(v value.Value, ok bool) value.Value {
	if !ok || r.failed {
		r.bug(nil)
		return nil
	}
	if r.fr.ts {
		return r.tsRead(v)
	}
	return v
}

func (h *stdHost) Invoke(fn value.Value, args ...value.Value) (value.Value, bool) {
	site := h.r.site
	v := h.r.invoke(fn, args, site)
	h.r.site = site
	return v, v != nil
}

// Coerce converts a value to the type a built-in's result holds (STDLIB.md §2.3).
func (h *stdHost) Coerce(v value.Value, t types.Type) (value.Value, bool) {
	c := h.r.coerce(v, t, nil)
	return c, c != nil
}

func (h *stdHost) Charge(n int) bool {
	return h.r.spend(n, func() source.Span { return h.r.site })
}

func (h *stdHost) Equal(a, b value.Value) (bool, bool) {
	return h.r.equal(a, b, func() source.Span { return h.r.site })
}

func (h *stdHost) Key(m *value.Map, k value.Value) value.Value {
	return h.r.keyFor(m, k)
}

func (h *stdHost) Remaining() int {
	return h.r.remaining()
}

func (h *stdHost) Fail(b *diag.Builder) {
	h.r.fail(b)
}

func (h *stdHost) Site() source.Span {
	return h.r.site
}

// Regexp compiles a regex literal once per evaluator; the checker validated it (E1114).
func (h *stdHost) Regexp(pattern string) *regexp.Regexp {
	e := h.r.ev
	re, ok := e.regexps[pattern]
	if !ok {
		re, _ = regexp.Compile(pattern)
		e.regexps[pattern] = re
	}
	return re
}
