package lock

// Order is a stable table's facts as AddTable checked them, and how they sort and merge: what
// a later AddTable of the same facts, spans aside, reuses, since neither looks at a span.
type Order struct {
	pkg     string
	facts   []Fact
	kept    []int  // the index in facts of each merged fact, in canonical order: the first of its kind
	retired []bool // whether each merged fact is retired: whether any fact it merged is
}

// orders reports o holding the facts of batch, spans aside, for package pkg.
func (o *Order) orders(pkg string, batch []Fact) bool {
	if o == nil || o.pkg != pkg || len(o.facts) != len(batch) {
		return false
	}
	for i, fact := range batch {
		fact.Span = o.facts[i].Span
		if fact != o.facts[i] {
			return false
		}
	}
	return true
}

// run is batch merged as o says: the facts o keeps, in canonical order, each with its span in
// batch and the retirement of the facts it merged.
func (o *Order) run(batch []Fact) []Fact {
	out := make([]Fact, len(o.kept))
	for i, k := range o.kept {
		out[i] = batch[k]
		out[i].Retired = o.retired[i]
	}
	return out
}

// merge checks the facts of batch in order and merges those before the first refused one,
// which is the error; the Order is nil with an error.
func (s *Sources) merge(batch []Fact) (*Order, error) {
	n, err := len(batch), error(nil)
	for i := range batch {
		if err = s.facts.valid(batch[i]); err != nil {
			n = i
			break
		}
	}
	o := &Order{pkg: s.facts.Package, facts: batch}
	for _, i := range sortedOrder(batch[:n]) {
		if last := len(o.kept) - 1; last >= 0 && compareFactPtrs(&batch[o.kept[last]], &batch[i]) == 0 {
			o.retired[last] = o.retired[last] || batch[i].Retired
			continue
		}
		o.kept, o.retired = append(o.kept, i), append(o.retired, batch[i].Retired)
	}
	s.facts.mergeRun(o.run(batch))
	if err != nil {
		return nil, err
	}
	return o, nil
}
