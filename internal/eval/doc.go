// Package eval is Canon's one evaluator: the interpreter, its step budget, freezing, poisoning,
// layer application, and the runs conformance needs: test calls and vectors (spec/EVALUATION.md).
//
// A Memo lets the evaluators of successive snapshots replay the entries whose inputs did not
// change. Epoch rule: every full check.Check starts a new epoch; only the programs of one
// Session.Recheck lineage share one.
package eval
