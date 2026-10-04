package ir

import "errors"

// ErrInternal is a stage-E invariant broken: a translated fn left without a body, and no error reported that explains it (decision 196).
var ErrInternal = errors.New("ir: internal error")

// fmtUntranslated names the translated fn ErrInternal is about.
const fmtUntranslated = "%w: translated fn %s has no body and no finding explains it"

// fmtEndlessChain names the stored fn whose receiver a result chain met again, past DECISIONS 284's refusal.
const fmtEndlessChain = "%w: stored fn %s met its own receiver's declaration below its results, though refused as cyclic"

// ErrFingerprint is a type the canon-fp v1 grammar cannot print: a malformed TypeRef, or a
// kind with no wire form, which stage E refuses first (E8151).
var ErrFingerprint = errors.New("ir: type has no canon-fp v1 form")

// ErrPattern is a pattern CompilePattern has no automaton for: one outside EVALUATION.md §11.3's portable subset, which check refuses first (E1904).
var ErrPattern = errors.New("ir: pattern outside the portable subset")

// fmtPatternParse and fmtPatternInst say why a pattern has no automaton; fmtNoText names a @text fn left without a value.
const (
	fmtPatternParse = "%w: %w"
	fmtPatternInst  = "%w: instruction %v has no automaton state"
	fmtNoText       = "%w: package %s: @text fn %s has no String value"
)
