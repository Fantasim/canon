package load

import (
	"context"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// wireHost is wire.Host for a decode this milestone can run without the evaluator (DECISIONS 173).
// span is every default's Prov.Span, the load.dir call's own, until the evaluator backs Default.
type wireHost struct {
	span source.Span
}

// Default reads f's default literally, a redundant "(...)" around it unwrapped as supported's
// gate does (defaultLiteralOK); with none, it is the load call's span (DECISIONS 173).
func (h wireHost) Default(_ context.Context, f *types.Field, _ wire.Instance, via *value.Prov) (value.Value, bool) {
	p := &value.Prov{Kind: value.ProvDefault, Span: h.span, Via: via}
	if f.Default == nil {
		return &value.None{T: f.Type, P: p}, true
	}
	switch n := unparen(f.Default).(type) {
	case *syntax.NoneLit:
		return &value.None{T: f.Type, P: p}, true
	case *syntax.BoolLit:
		return &value.Bool{V: n.Value, P: p}, true
	case *syntax.IntLit:
		return &value.Int{V: n.Value.Int64(), T: baseType(f.Type), P: p}, true
	case *syntax.DurationLit:
		return &value.Dur{Ms: n.Millis, P: p}, true
	case *syntax.StringLit:
		s, _ := plainString(n)
		return &value.Str{V: s, T: baseType(f.Type), P: p}, true
	default:
		return nil, false
	}
}

// Deref is never reached: supported refuses a decode whose type has a ref field.
func (wireHost) Deref(context.Context, *value.Ref) (*value.Record, bool) {
	return nil, false
}
