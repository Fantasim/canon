package eval

// Charge is the steps charged to one root, named in its package (EVALUATION.md §12.2).
type Charge struct {
	Pkg, Name string
	Steps     int64
}

// Charged is every root's steps so far, in the order each was first charged.
func (e *Evaluator) Charged() []Charge {
	out := make([]Charge, 0, len(e.order))
	for _, c := range e.order {
		out = append(out, Charge{Pkg: c.pkg, Name: c.name, Steps: e.spent[c]})
	}
	return out
}
