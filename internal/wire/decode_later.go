package wire

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// pending is a record whose waiting fields are decoded in the second pass, once the whole value
// has its other fields, so a discriminant reads decoded entries of any collection in it.
type pending struct {
	o          *object
	fields     []*types.Field
	rv         *value.Record
	fr         *frame
	key        *types.Field // a keyed-list element's key field, never deferred
	trail      []step       // where the record sits, for the refs its waiting fields bind
	waiting    []int        // the fields of the second pass, in declaration order
	incomplete []bool       // the fields that wait or may hold a record that does
	kids       []*pending   // the pending records inside its other fields
	via        value.Value  // the ref it last needed a waiting field through
	state      entryState
	ok         bool
}

// pend is the pending state of a record decoded in the first pass whose type may leave fields
// waiting; nil in the second pass, which decodes every field at once.
func (r *run) pend(o *object, fields []*types.Field, rv *value.Record, fr *frame) *pending {
	key := r.entryKey
	r.entryKey = nil
	if r.pass2 || !r.mayWait(rv.T, fields) {
		return nil
	}
	return &pending{o: o, fields: fields, rv: rv, fr: fr, key: key, incomplete: make([]bool, len(fields)), state: entryWaiting}
}

// keep queues p for the second pass when it deferred a field; its kids are the pending records
// queued since first, the start of its fields.
func (r *run) keep(p *pending, first int) {
	if p == nil || len(p.waiting) == 0 {
		return
	}
	p.kids = r.later[first:]
	r.later = append(r.later, p)
	r.waiting[p.rv] = p
}

// mayWait reports a record type one field of which may wait or hold a record that does.
func (r *run) mayWait(t types.Type, fields []*types.Field) bool {
	found, ok := r.deps[t]
	if !ok {
		found = slices.ContainsFunc(fields, func(f *types.Field) bool { return len(f.DependsOn) > 0 || r.holdsApp(f.Type) })
		r.deps[t] = found
	}
	return found
}

// deferred reports that field i of p waits for the second pass, its key claimed meanwhile: its
// own type reads an earlier field or is an application, or it is absent and its default reads a
// field that waits or may hold a record that does.
func (r *run) deferred(p *pending, i int, t types.Type) bool {
	f := p.fields[i]
	p.incomplete[i] = r.holdsApp(f.Type)
	if len(f.WirePath) == 0 || f.Input != nil || f.Inline || f.Pairs != nil || f == p.key {
		return false
	}
	if len(f.DependsOn) > 0 || direct(f.Type) {
		r.lookup(p.o, f.WirePath, t)
		return true
	}
	if f.Default == nil || r.d.Host == nil || !slices.ContainsFunc(r.d.Host.Reads(f, p.fields), func(j int) bool { return j < i && p.incomplete[j] }) {
		return false
	}
	m, ok := r.lookup(p.o, f.WirePath, t)
	p.incomplete[i] = ok && m == nil
	return p.incomplete[i]
}

// direct reports a type that is an application itself, through optionals, lists, maps and
// literal unions: its value is bound where its field sits.
func direct(t types.Type) bool {
	switch x := t.Base().(type) {
	case *types.TypeAppType, *types.AppliedRecord:
		return true
	case *types.OptionalType:
		return direct(x.Elem)
	case *types.ListType:
		return direct(x.Elem)
	case *types.MapType:
		return direct(x.Key) || direct(x.Value)
	case *types.DepMapType:
		return direct(x.Value)
	case *types.LitUnionType:
		return direct(x.Of)
	}
	return false
}

// holdsApp reports a type whose values may hold a dependent value or an applied record; each
// type is scanned once.
func (r *run) holdsApp(t types.Type) bool {
	found, ok := r.apps[t]
	if !ok {
		found = (&appScan{seen: map[types.Type]bool{}}).visit(t)
		r.apps[t] = found
	}
	return found
}

// appScan walks a type's values without crossing refs, each type once.
type appScan struct {
	seen map[types.Type]bool
}

func (s *appScan) visit(t types.Type) bool {
	if t == nil || s.seen[t] {
		return false
	}
	s.seen[t] = true
	switch x := t.Base().(type) {
	case *types.TypeAppType, *types.AppliedRecord:
		return true
	case *types.OptionalType:
		return s.visit(x.Elem)
	case *types.ListType:
		return s.visit(x.Elem)
	case *types.TableType:
		return s.visit(x.Elem)
	case *types.MapType:
		return s.visit(x.Key) || s.visit(x.Value)
	case *types.DepMapType:
		return s.visit(x.Value)
	case *types.LitUnionType:
		return s.visit(x.Of)
	case *types.VariantType:
		return slices.ContainsFunc(x.Cases, func(c *types.CaseType) bool { return s.fields(c.Fields) })
	case *types.RecordType:
		return s.fields(x.Fields)
	}
	return false
}

func (s *appScan) fields(fs []*types.Field) bool {
	return slices.ContainsFunc(fs, func(f *types.Field) bool { return s.visit(f.Type) })
}

// remember keeps Decoder.Coll's entries by key: a ref into the collection being loaded reads them (EVALUATION.md §3.2).
func (r *run) remember(entries []value.Value) {
	r.local = map[value.Key]*value.Record{}
	for _, e := range entries {
		if rec, ok := e.(*value.Record); ok && rec.Ident != nil {
			r.local[rec.Ident.Key] = rec
		}
	}
}

// finishAll is the second pass: every pending record, in the order the first pass queued them.
func (r *run) finishAll() bool {
	r.pass2 = true
	ok := true
	for _, p := range r.later {
		ok = r.finishFrom(p) && ok
	}
	return ok
}

// want is a record an attempt needs, and the ref it was reached through.
type want struct {
	p   *pending
	via value.Value
}

// finishFrom finishes p on an explicit stack: first its kids, then every record an attempt
// needs, the attempt then retried; one needed while on the stack is a cycle.
func (r *run) finishFrom(p *pending) bool {
	stack := []*pending{p}
	for len(stack) > 0 {
		top := stack[len(stack)-1]
		if top.state == entryDone {
			stack = stack[:len(stack)-1]
			continue
		}
		wants := r.wantsOf(top)
		if i := slices.IndexFunc(wants, func(w want) bool { return w.p.state == entryRunning }); i >= 0 {
			r.cycle(wants[i].via)
			top.state, top.ok = entryDone, false
			continue
		}
		if len(wants) == 0 {
			top.state = entryDone
		}
		for _, w := range slices.Backward(wants) {
			stack = append(stack, w.p)
		}
	}
	return p.ok
}

// wantsOf is what top waits for: its kids still to finish, else every record an attempt needs;
// none once top is done.
func (r *run) wantsOf(top *pending) []want {
	var kids []want
	for _, k := range top.kids {
		if k.state != entryDone {
			kids = append(kids, want{p: k, via: k.via})
		}
	}
	if len(kids) > 0 {
		return kids
	}
	return r.attempt(top)
}

// attempt decodes p's waiting fields where p sits, to the end even past a field still waiting
// elsewhere, so it collects every record it needs; then it is undone, the host taking back what
// it did (Host.Savepoint), and retried once they are finished.
func (r *run) attempt(p *pending) []want {
	p.state = entryRunning
	end := r.savepoint()
	saved, failed := r.trail, p.fr.failed
	r.needs, r.needed = nil, map[*pending]bool{}
	ok := true
	for _, i := range p.waiting {
		f := p.fields[i]
		r.trail = append(slices.Clip(p.trail), step{field: f.Name})
		ok = r.field(p.o, f, i, p.rv, p.fr) && ok
		p.fr.failed = p.fr.failed || !ok
	}
	r.trail = saved
	needs := r.needs
	r.needs, r.needed = nil, nil
	end(len(needs) > 0)
	if len(needs) == 0 {
		p.ok = ok
		return nil
	}
	p.via, p.fr.failed = needs[0].via, failed
	for _, i := range p.waiting {
		p.rv.Fields[i], p.rv.Set[i] = nil, false
	}
	return needs
}

// savepoint is the host's mark of an attempt, or nothing without a host.
func (r *run) savepoint() func(undo bool) {
	if r.d.Host == nil {
		return func(bool) {}
	}
	return r.d.Host.Savepoint()
}

// settle reports that field i of rec may be read now; when it still waits, the attempt reading
// it notes the record it needs and goes on.
func (r *run) settle(rec *value.Record, i int, via value.Value) bool {
	p := r.waiting[rec]
	if r.needed == nil || p == nil || p.state == entryDone || !slices.Contains(p.waiting, i) {
		return true
	}
	if !r.needed[p] {
		r.needed[p] = true
		r.needs = append(r.needs, want{p: p, via: via})
	}
	return false
}

// cycle has the host report a record needed to decode itself at the ref that closed the cycle.
func (r *run) cycle(via value.Value) {
	ref, isRef := via.(*value.Ref)
	switch {
	case !isRef:
		r.misuse(ErrShape, nil)
	case r.d.Host == nil:
		r.misuse(ErrNoHost, ref.T)
	default:
		r.d.Host.Cycle(r.ctx, ref)
	}
	r.failed = true
}

// localEntry is the decoded entry of the whole collection a ref names; mine reports a ref into
// that collection once its entries are known, nil then meaning the key names none.
func (r *run) localEntry(ref *value.Ref) (rec *value.Record, mine bool) {
	rt, ok := ref.T.Base().(*types.RefType)
	if !ok || r.local == nil || rt.Target != r.d.Coll {
		return nil, false
	}
	return r.local[ref.Key], true
}

// entryOf reports that elem is the entry type of Decoder.Coll decoded as the whole value.
func (r *run) entryOf(elem types.Type, sc wscope) bool {
	if !sc.root || r.d.Coll == nil {
		return false
	}
	switch elem.Base().(type) {
	case *types.RecordType, *types.AppliedRecord:
		return true
	}
	return false
}
