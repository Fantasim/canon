// Package std is the standard library the evaluator calls (spec/STDLIB.md): the methods of
// sequences, keyed collections, maps, strings and ranges, the free functions (conversions,
// math, graphs), the canonical text form and format specs, each with its step cost and its
// errors. It reaches the evaluator only through Host, so eval imports std and never the reverse.
package std
