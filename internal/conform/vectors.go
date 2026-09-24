package conform

import (
	"context"
	"errors"
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// filler fills the translated fns of one package from its test calls; ts asks for TS expectations (CONFORMANCE.md §4).
type filler struct {
	ev      Evaluator
	bag     *diag.Bag
	ts      bool
	calls   []Call
	unknown bool // a test of the package has an error: its calls cannot be known, so no E9008
	errored bool // the bags held an error before Fill: an evaluation with no outcome read a poisoned value
}

// input is one vector before evaluation: the receiver evaluated, its projection, the arguments.
type input struct {
	full value.Value
	recv []value.Value
	args []value.Value
}

// receiver is a distinct projected receiver with the first full receiver that had it.
type receiver struct {
	full  value.Value
	reads []value.Value
}

// fill selects s's vectors and computes their results; a method no test calls is E9008 (CONFORMANCE.md §6).
func (f *filler) fill(ctx context.Context, s *site) error {
	if !supported(s.sig) {
		return nil // ir reports E9006
	}
	calls := ownCalls(f.calls, s.obj)
	tests, recvs, err := s.receivers(calls, f.selfFn(ctx, s))
	if err != nil {
		return err
	}
	if s.method && len(calls) == 0 {
		if !f.unknown { // no cascade: a broken test's error is reported already (meta/decisions/log-2026-09-24.md, build wiring)
			diag.E9008.At(s.span(), s.owner, s.fn.Name, s.pkg).Report(f.bag)
		}
		return nil
	}
	if s.method && len(recvs) == 0 {
		return nil // every call skipped (meta/decisions/log-2026-09-24.md, conform N1)
	}
	if !s.method {
		recvs = []receiver{{}}
	}
	inputs, err := s.selectVectors(tests, recvs, calls)
	if err != nil {
		return err
	}
	s.fn.Vectors, err = f.expect(ctx, s, inputs)
	return err
}

// supported reports a signature whose every parameter has candidates (CONFORMANCE.md §6.2).
func supported(sig *types.FuncType) bool {
	for _, p := range sig.Params {
		switch t := p.Base().(type) {
		case *types.EnumType:
		case types.Basic:
			if basicRules[t.K] == nil {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// ownCalls is the calls of fn, in order.
func ownCalls(calls []Call, fn check.Object) []Call {
	var out []Call
	for _, c := range calls {
		if c.Fn == fn {
			out = append(out, c)
		}
	}
	return out
}

// receivers is each test call as a vector, and the distinct projected receivers in first-seen order, but a call skipped by conform N1 (CONFORMANCE.md §6.1).
func (s *site) receivers(calls []Call, method selfFn) ([]input, []receiver, error) {
	tests := make([]input, 0, len(calls))
	var recvs []receiver
	seen := map[string]bool{}
	for _, c := range calls {
		reads, key, err := project(c.Recv, s.fn.Reads, method)
		if errors.Is(err, errFailedRead) {
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf(fmtNamed, err, s.label)
		}
		tests = append(tests, input{full: c.Recv, recv: reads, args: c.Args})
		if s.method && !seen[key] {
			seen[key] = true
			recvs = append(recvs, receiver{full: c.Recv, reads: reads})
		}
	}
	return tests, recvs, nil
}

// selectVectors is the test vectors, then each receiver's, without repeats (CONFORMANCE.md §6.4).
func (s *site) selectVectors(tests []input, recvs []receiver, calls []Call) ([]input, error) {
	var out []input
	seen := map[string]bool{}
	keep := func(in input) error {
		key, ok := keysOf(append(append([]value.Value(nil), in.recv...), in.args...))
		if !ok { // DECISIONS 204: ir admits no such read or parameter
			return fmt.Errorf(fmtNamed, ErrUnsupported, s.label)
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, in)
		}
		return nil
	}
	for _, in := range tests {
		if err := keep(in); err != nil {
			return nil, err
		}
	}
	for _, r := range recvs {
		for _, in := range s.generated(r, calls) {
			if err := keep(in); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// generated is receiver r's vectors: its parameters' candidates combined (CONFORMANCE.md §6.3).
func (s *site) generated(r receiver, calls []Call) []input {
	lists := make([][]value.Value, len(s.sig.Params))
	sizes := make([]int, len(lists))
	for i := range lists {
		lists[i] = s.candidates(i, r.reads, calls)
		sizes[i] = len(lists[i])
	}
	tuples := combine(sizes)
	out := make([]input, len(tuples))
	for n, t := range tuples {
		args := make([]value.Value, len(t))
		for k, idx := range t {
			args[k] = lists[k][idx]
		}
		out[n] = input{full: r.full, recv: r.reads, args: args}
	}
	return out
}
