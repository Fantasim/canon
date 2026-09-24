package jsongen

import (
	"fmt"
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// selection is the emit's values in listed order, else every public value (WIRE.md §8.1).
func selection(p *ir.Package, e *ir.Emit) ([]*ir.Value, error) {
	byName := make(map[string]*ir.Value, len(p.Values))
	for _, v := range p.Values {
		if v == nil || byName[v.Name] != nil {
			return nil, fmt.Errorf("%w: public values of %s", ErrValues, p.Name)
		}
		byName[v.Name] = v
	}
	if len(e.Values) == 0 {
		return p.Values, nil
	}
	out := make([]*ir.Value, 0, len(e.Values))
	listed := make(map[string]bool, len(e.Values))
	for _, name := range e.Values {
		if byName[name] == nil || listed[name] {
			return nil, fmt.Errorf(fmtNamed, ErrValues, name)
		}
		listed[name] = true
		out = append(out, byName[name])
	}
	return out, nil
}

// layout is each value's path in Dir: FileName in file mode, else `<value>.json` (decision 127).
func layout(e *ir.Emit, values []*ir.Value) ([]string, error) {
	if e.FileName == "" {
		paths := make([]string, len(values))
		for i, v := range values {
			paths[i] = v.Name + ir.JSONExt
		}
		return paths, nil
	}
	if !strings.HasSuffix(e.FileName, ir.JSONExt) || path.Base(e.FileName) != e.FileName || len(values) != 1 {
		return nil, fmt.Errorf(fmtCount, ErrFileMode, e.FileName, len(values))
	}
	return []string{e.FileName}, nil
}

// dataMode holds the data mode link of E8153 for this emit's files (WIRE.md §8.1).
func dataMode(p *ir.Package, e *ir.Emit, values []*ir.Value, paths []string) error {
	written := make(map[string]string, len(values))
	for i, v := range values {
		written[v.Name] = paths[i]
	}
	for _, o := range p.Emits {
		if o == nil || o == e || !codeTarget(o.Target) || o.Mode != ir.ModeData {
			continue
		}
		if err := linked(written, emitted(p, o)); err != nil {
			return err
		}
	}
	var reload []string
	for _, v := range p.Values {
		if v.Reload {
			reload = append(reload, v.Name)
		}
	}
	return linked(written, reload)
}

// linked checks that each name is written, to `<name>.json`.
func linked(written map[string]string, names []string) error {
	for _, name := range names {
		got, ok := written[name]
		if !ok {
			return fmt.Errorf(fmtNamed, ErrDataMode, name)
		}
		if got != name+ir.JSONExt {
			return fmt.Errorf(fmtFile, ErrDataMode, name, got)
		}
	}
	return nil
}

// emitted is the values a code emit selects: its `values`, else every public value.
func emitted(p *ir.Package, o *ir.Emit) []string {
	if len(o.Values) > 0 {
		return o.Values
	}
	names := make([]string, len(p.Values))
	for i, v := range p.Values {
		names[i] = v.Name
	}
	return names
}

// codeTarget is a target with modes (CODEGEN.md §2.1).
func codeTarget(t ir.Target) bool {
	switch t {
	case ir.TargetGo, ir.TargetCpp, ir.TargetTS:
		return true
	default:
		return false
	}
}
