package eval

// Charge is the steps one root charged to Pkg's budget; an instance check run for Pkg's value may be declared elsewhere (EVALUATION.md §12.2).
type Charge struct {
	Pkg, Name string
	Steps     int64
}

// Charged is every root's steps so far, in the order each was first charged.
func (e *Evaluator) Charged() []Charge {
	out := make([]Charge, 0, len(e.order))
	for _, c := range e.order {
		out = append(out, Charge{Pkg: c.pkg, Name: c.name, Steps: e.spentOn(c)})
	}
	return out
}
