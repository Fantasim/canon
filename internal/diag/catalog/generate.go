package catalog

import (
	"bytes"
	"cmp"
	"fmt"
	"go/format"
	"slices"
	"strconv"
	"strings"
	"text/template"
)

// File is one generated file: its name inside internal/diag and its gofmt-formatted bytes.
type File struct {
	Name string
	Data []byte
}

type genFile struct {
	Types []genConst
	Kinds []genConst
	Defs  []genDef
	Calls []genCall
}

type genConst struct {
	Const, Name, Word string
}

type genDef struct {
	Index                        int
	Code, Severity, Package, Doc string
	Runtime                      bool
	Variants                     []genVariant
}

type genVariant struct {
	Index                        int
	Name, Method, Quoted, Source string
	Args                         []genArg
}

type genArg struct {
	Name, Const, GoType, Sample string
}

type genCall struct {
	Code, Method string
	Samples      []string
}

// Generate renders codes.go and the constructor table of its test (ERRORS.md §2.2).
func (c *Catalog) Generate() ([]File, error) {
	view := c.view()
	out := make([]File, 0, len(generated))
	for _, g := range generated {
		tpl, err := template.New(g.name).Parse(g.text)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", errGenerate, g.name, err)
		}
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, view); err != nil {
			return nil, fmt.Errorf("%w: %s: %w", errGenerate, g.name, err)
		}
		src, err := format.Source(buf.Bytes())
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", errGenerate, g.name, err)
		}
		out = append(out, File{Name: g.name, Data: src})
	}
	return out, nil
}

// view is the template data: the codes sorted by code (the order of diag.Registry).
func (c *Catalog) view() genFile {
	goTypes := map[string]genConst{}
	var v genFile
	for _, t := range c.ArgTypes {
		gc := genConst{Const: argTypePrefix + t.Name, Name: t.Name, Word: strings.ReplaceAll(t.GoType, diagQualifier, "")}
		goTypes[t.Name] = gc
		v.Types = append(v.Types, gc)
	}
	for _, k := range c.Kinds {
		v.Kinds = append(v.Kinds, genConst{Const: kindPrefix + k.Name, Name: k.Name, Word: k.Word})
	}
	codes := slices.Clone(c.Codes)
	slices.SortFunc(codes, func(a, b Code) int { return cmp.Compare(a.ID, b.ID) })
	for i, code := range codes {
		d := c.genDef(i, code, goTypes)
		v.Defs = append(v.Defs, d)
		if !d.Runtime {
			v.Calls = append(v.Calls, calls(d)...)
		}
	}
	return v
}

func (c *Catalog) genDef(index int, code Code, goTypes map[string]genConst) genDef {
	d := genDef{
		Index: index, Code: code.ID, Severity: upperFirst(code.Severity), Package: code.Package,
		Doc: fmt.Sprintf(fmtCodeDoc, code.ID, code.Meaning, code.Owner), Runtime: code.Severity == severityRuntime,
	}
	for i, m := range code.Messages {
		gv := genVariant{
			Index: i, Name: m.Variant, Method: constructorPrefix + upperFirst(m.Variant),
			Quoted: strconv.Quote(m.Template), Source: m.Template,
		}
		for _, a := range m.Args {
			t := goTypes[a.Type]
			sample := samplePrefix + a.Type
			if a.Type == kindType {
				sample = kindPrefix + firstKind(c, code.ID)
			}
			gv.Args = append(gv.Args, genArg{Name: a.Name, Const: t.Const, GoType: t.Word, Sample: sample})
		}
		d.Variants = append(d.Variants, gv)
	}
	return d
}

// calls lists one constructor call per variant of a reported code.
func calls(d genDef) []genCall {
	out := make([]genCall, 0, len(d.Variants))
	for _, v := range d.Variants {
		call := genCall{Code: d.Code, Method: v.Method}
		for _, a := range v.Args {
			call.Samples = append(call.Samples, a.Sample)
		}
		out = append(out, call)
	}
	return out
}

// upperFirst turns a lowerCamel name into UpperCamel.
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
