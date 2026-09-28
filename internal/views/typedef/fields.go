package typedef

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/encode"
)

// field is a field definition keyed key (VIEWMODEL.md 12.3 Field): `required` when it has no
// default, is not optional and is not an input; its default, help, deprecation, wire mapping and
// input.
func (s *Types) field(decl types.Type, f *types.Field, key []string) vm.Field {
	out := vm.Field{
		Name:     f.Name,
		Type:     s.expr(at{decl: decl, enc: f.Enc}, f.Type),
		Required: f.Default == nil && f.Input == nil && f.Type.Base().Kind() != types.Optional,
		Default:  s.defaultOf(decl, f),
		Help:     s.help(f.Doc, s.in.Index.Field(f), key...),
		Wire:     wire(f),
		Stable:   f.Stable,
	}
	if f.Deprecated != nil {
		out.Deprecated = s.deprecated(f.Deprecated.Why, key)
	}
	if f.Input != nil {
		out.Input = &vm.FieldInput{Env: f.Input.Env}
	}
	return out
}

// deprecated is a deprecation's reason keyed under key: its key when catalogued, else a neutral
// text, `""` for no reason (the flag kept, VIEWMODEL.md J9).
func (s *Types) deprecated(why string, key []string) vm.TextRef {
	if ref := s.in.Texts.Text(s.pkg, why, append(slices.Clip(key), syntax.AnnDeprecated)...); ref != (vm.TextRef{}) {
		return ref
	}
	return vm.TextRef{Text: &why}
}

// wire is a field's wire mapping (VIEWMODEL.md 12.3): its key, and when written its path, unit,
// `none` value, inline, int, bits and pairs; a pairs field is named by its first template.
func wire(f *types.Field) vm.Wire {
	w := vm.Wire{Name: f.Wire, Inline: f.Inline, Int: f.Enc == types.EncInt, Bits: f.Enc == types.EncBits}
	if len(f.WirePath) > 1 {
		w.Path = strings.Join(f.WirePath, dot)
	}
	if unitWritten(f) {
		w.Unit = f.Unit.String()
	}
	if f.NoneWire != nil {
		w.None = json.RawMessage(f.NoneWire)
	}
	if f.Pairs != nil {
		w.Name = f.Pairs.Keys[0]
		w.Pairs = &vm.WirePairs{Keys: f.Pairs.Keys[:], Slots: f.Pairs.Slots}
	}
	return w
}

// unitWritten reports a field with @json(unit:), whose Unit is then written even when `ms`.
func unitWritten(f *types.Field) bool {
	for _, a := range f.Annotations {
		if a.Name == nil || a.Name.Name != syntax.AnnJSON {
			continue
		}
		for _, arg := range a.Args {
			if arg.Name != nil && arg.Name.Name == syntax.ArgUnit {
				return true
			}
		}
	}
	return false
}

// defaultOf is a field's default in the value encoding (J10), `{"computed": true}` when it reads
// the record's fields or parameters (TYP-15); nil without a default, a folder, or a fold.
func (s *Types) defaultOf(decl types.Type, f *types.Field) json.RawMessage {
	if f.Default == nil {
		return nil
	}
	info := s.in.Program.Info
	if readsInstance(info, f.Default, recordParams(info, decl)) {
		return json.RawMessage(computedDefault)
	}
	owner := s.objects[ownerKey(decl)]
	if s.in.Fold == nil || owner == nil {
		return nil
	}
	v, ok := s.in.Fold.Fold(s.ctx, owner, f.Default, info)
	if !ok {
		return nil
	}
	return encode.Value(v)
}

// ownerKey is the declaration holding the fields of the record or case decl.
func ownerKey(decl types.Type) any {
	if c, ok := decl.(*types.CaseType); ok {
		return c.Variant
	}
	return decl
}

// recordParams are the check objects of a record's own value parameters; none for a case.
func recordParams(info *check.Info, decl types.Type) map[check.Object]bool {
	r, ok := decl.(*types.RecordType)
	if !ok || r.Decl == nil {
		return nil
	}
	out := map[check.Object]bool{}
	for _, p := range r.Decl.Params {
		if o := info.Defs[p.Name]; o != nil {
			out[o] = true
		}
	}
	return out
}

// readsInstance reports an expression naming a field or one of params (TYPES.md §15, TYP-15).
func readsInstance(info *check.Info, e syntax.Expr, params map[check.Object]bool) bool {
	found := false
	syntax.Inspect(e, func(n syntax.Node) bool {
		if id, ok := n.(*syntax.IdentExpr); ok {
			o := info.Uses[id]
			found = found || o != nil && (o.Kind() == check.ObjField || params[o])
		}
		return !found
	})
	return found
}
