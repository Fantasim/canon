// Package verify is stage B of a build: it walks each evaluated top-level value against its
// declared type and checks refinements, sized ranges, finiteness, retired members and cases,
// refs, keyed-list keys, stable values, @codes codes and assets (spec/EVALUATION.md, TYPES.md).
// Each soft finding marks the value it is about invalid in the evaluator.
package verify
