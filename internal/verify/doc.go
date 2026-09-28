// Package verify is stage B of a build: it walks each evaluated top-level value against its
// declared type and checks refinements, sized ranges, finiteness, retired members and cases,
// refs, keyed-list keys, stable values, @codes codes, assets and dependent values, each
// converted to the type its arguments compute (spec/EVALUATION.md, TYPES.md).
// Each soft finding marks the value it is about invalid in the evaluator.
package verify
