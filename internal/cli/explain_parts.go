package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/api/vm"
)

// part is one part of a value: a value the analysis computed, or an input field, which has none, by its path and environment variable (EVALUATION.md §11.2).
type part struct {
	value *canon.Value
	path  string
	env   string
}

// partSource lists the parts of values in declaration order, input fields from the view model of the declaring package, each read once (VIEWMODEL.md §12.1 `input.env`).
type partSource struct {
	ctx    context.Context
	p      *canon.Project
	models map[string]*vm.ViewModel
}

func newPartSource(ctx context.Context, p *canon.Project) *partSource {
	return &partSource{ctx: ctx, p: p, models: map[string]*vm.ViewModel{}}
}

// parts is v's parts, each input field where its record or variant case declares it (CLI.md §3.7).
func (s *partSource) parts(v *canon.Value) ([]part, error) {
	fields, err := s.fields(v)
	if err != nil {
		return nil, err
	}
	return interleave(v, fields), nil
}

// interleave is v's children with the input fields among fields, in declaration order; a child
// no field names (none is written for a pseudo-field) follows.
func interleave(v *canon.Value, fields []vm.Field) []part {
	children := v.Children()
	out := make([]part, 0, len(children)+len(fields))
	next := 0
	for _, f := range fields {
		switch {
		case f.Input != nil:
			out = append(out, part{path: v.Path + string(fieldMark) + f.Name, env: f.Input.Env})
		case next < len(children) && strings.HasSuffix(children[next].Path, string(fieldMark)+f.Name):
			out = append(out, part{value: children[next]})
			next++
		}
	}
	for _, c := range children[next:] {
		out = append(out, part{value: c})
	}
	return out
}

// fields are the declared fields of v's record, or of its variant's current case; none for any
// other value.
func (s *partSource) fields(v *canon.Value) ([]vm.Field, error) {
	if (v.Kind != canon.KindRecord && v.Kind != canon.KindVariant) || len(v.Type.VM) == 0 {
		return nil, nil
	}
	var t vm.TypeExpr
	if err := json.Unmarshal(v.Type.VM, &t); err != nil {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	for t.Kind == vmOptional && t.Of != nil {
		t = *t.Of
	}
	if (t.Kind != vmRecord && t.Kind != vmVariant) || t.Ref == "" {
		return nil, nil
	}
	def, err := s.definition(t.Ref)
	if err != nil || def == nil {
		return nil, err
	}
	if def.Kind == vmRecord {
		return def.Fields, nil
	}
	name, _ := v.Case()
	for _, c := range def.Cases {
		if c.Name == name {
			return c.Fields, nil
		}
	}
	return nil, nil
}

// definition is the type ref names, from the view model of its package: the ref is the
// package's name, a dot and the type's name.
func (s *partSource) definition(ref string) (*vm.TypeDef, error) {
	dot := strings.LastIndexByte(ref, fieldMark)
	if dot < 0 {
		return nil, nil
	}
	model, err := s.model(ref[:dot])
	if err != nil {
		return nil, err
	}
	def, ok := model.Types[ref]
	if !ok {
		return nil, nil
	}
	return &def, nil
}

// model is pkg's view model, read once.
func (s *partSource) model(pkg string) (*vm.ViewModel, error) {
	if m, ok := s.models[pkg]; ok {
		return m, nil
	}
	m := &vm.ViewModel{} // a type of no package of the project has no input fields to list
	got, err := s.p.ViewModel(s.ctx, pkg)
	if err != nil && !errors.Is(err, canon.ErrUnknownPackage) {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	if err == nil {
		if err := got.Decode(m); err != nil {
			return nil, fmt.Errorf(fmtWrap, err)
		}
	}
	s.models[pkg] = m
	return m, nil
}
