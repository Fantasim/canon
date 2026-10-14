package eval

import (
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// ADR-0011, DECISIONS 379: a memo fingerprint hashes the position mark, its Via's included.
func TestMemoFingerprintPosition(t *testing.T) {
	e := newEvaluator(check.Bags{}, Options{})
	at := source.Span{File: 1, Start: 2, End: 3}
	plain := &value.Int{V: 1, T: types.IntType, P: &value.Prov{Kind: value.ProvComputed, Span: at}}
	marked := &value.Int{V: 1, T: types.IntType, P: &value.Prov{Kind: value.ProvComputed, Position: true, Span: at}}
	if e.fingerprintOf(plain).fp == e.fingerprintOf(marked).fp {
		t.Fatal("a marked and an unmarked value share a fingerprint")
	}
	twin := &value.Int{V: 1, T: types.IntType, P: &value.Prov{Kind: value.ProvComputed, Position: true, Span: at}}
	if e.fingerprintOf(twin).fp != e.fingerprintOf(marked).fp {
		t.Fatal("two equally marked values differ in fingerprint")
	}
	via := &value.Prov{Kind: value.ProvSpread, Span: at, Via: marked.P}
	viaPlain := &value.Prov{Kind: value.ProvSpread, Span: at, Via: plain.P}
	if e.fingerprintOf(&value.Int{V: 1, T: types.IntType, P: via}).fp == e.fingerprintOf(&value.Int{V: 1, T: types.IntType, P: viaPlain}).fp {
		t.Fatal("the mark of a provenance's Via is not hashed")
	}
}
