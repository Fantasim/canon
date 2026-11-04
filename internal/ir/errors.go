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

// fmtPatternParse and fmtPatternInst say why a pattern has no automaton.
const fmtPatternParse, fmtPatternInst = "%w: %w", "%w: instruction %v has no automaton state"

const cppNames, cppByWire, cppCodeAt, cppRows = "Names", "ByWire", "CodeAt", "Rows" // .gen.cpp tables: an enum's k<E>Names, k<E>ByWire, k<E>CodeAt (CODEGEN.md §5.2), a lookup's k<C><Lookup>Rows (§5.9)

// PartLines is the most lines between a generated function's braces, a part's own `return` and call lines included: the budget every target cuts a body that grows with the data's rows or a record's fields to (CODEGEN.md §2.11, DECISIONS 435).
const PartLines = 1000

const goLookupKeys, goLookupAt = "lookupKeys", "lookupAt" // a lookup's enum tables lookupKeys_<E>, lookupAt_<E> (CODEGEN.md §5.10)
