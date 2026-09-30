package wire

import (
	"context"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Elems keeps a load.dir's record elements apart, one per file, which Dir then decodes no more (NFR-02).
type Elems interface {
	// Kept is the element kept for a file's selection, and whether a table's entry is retired;
	// false: the element is decoded.
	Kept(sel Selection) (rec *value.Record, retired, ok bool)
	// Start begins decoding a file's element; done is told the element (nil when it failed or is
	// not a record), whether a table's entry is retired, and whether decoding it did nothing but
	// make it: no finding, nothing left for the second pass, no host call but Default and Reads.
	Start(sel Selection) (done func(rec *value.Record, retired, pure bool))
}

// watchedHost is a Host counting the calls an element decoded purely never makes.
type watchedHost struct {
	Host
	calls *int
}

func (h watchedHost) Deref(ctx context.Context, r *value.Ref) (*value.Record, bool) {
	*h.calls++
	return h.Host.Deref(ctx, r)
}

func (h watchedHost) Bind(rec *value.Record, params map[*types.Param]value.Value) {
	*h.calls++
	h.Host.Bind(rec, params)
}

func (h watchedHost) Cycle(ctx context.Context, r *value.Ref) {
	*h.calls++
	h.Host.Cycle(ctx, r)
}

func (h watchedHost) Savepoint() func(undo bool) {
	*h.calls++
	return h.Host.Savepoint()
}

// watch makes r ask d.Elems, counting its host's calls; without a host r asks nothing.
func (r *run) watch(d *Decoder) {
	if d.Elems == nil || d.Host == nil {
		return
	}
	watched := *d
	watched.Host = watchedHost{Host: d.Host, calls: &r.hosted}
	r.d, r.elems = &watched, d.Elems
}

// elemWatch is what a run had done when an element's decoding began. hosted and failed only back
// the others up: a host call but a default comes with a field that waits (bar an inline or key
// one), and a failed default leaves no element.
type elemWatch struct {
	reported, hosted, later int
	failed                  bool
}

func (r *run) watching() elemWatch {
	return elemWatch{reported: r.reported, hosted: r.hosted, later: len(r.later), failed: r.failed}
}

// pure reports that decoding rec since w did nothing but make it.
func (r *run) pure(w elemWatch, rec *value.Record) bool {
	return rec != nil && r.err == nil && r.watching() == w
}

// dirElement is a load.dir file's element: the one kept, else decoded and offered (WIRE.md §6.5).
func (r *run) dirElement(sel Selection, t types.Type, sc wscope) value.Value {
	if r.elems == nil || !sc.root || sel.Star {
		return r.element(sel, t, sc.inner())
	}
	if rec, _, ok := r.elems.Kept(sel); ok {
		return rec
	}
	done, w := r.elems.Start(sel), r.watching()
	v := r.element(sel, t, sc.inner())
	rec, _ := v.(*value.Record)
	done(rec, false, r.pure(w, rec))
	return v
}

// dirRow is a load.dir file's table row and whether it is retired, kept or decoded (WIRE.md §6.5).
func (r *run) dirRow(sel Selection, t types.Type) (*value.Record, bool) {
	if r.elems == nil {
		return r.row(sel.Node, t, r.rootFrame())
	}
	if rec, retired, ok := r.elems.Kept(sel); ok {
		return rec, retired
	}
	done, w := r.elems.Start(sel), r.watching()
	rec, retired := r.row(sel.Node, t, r.rootFrame())
	done(rec, retired, r.pure(w, rec))
	return rec, retired
}
