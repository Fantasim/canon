package ir_test

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// dump prints a package's IR deterministically, every field a generator reads, for goldens.
func dump(p *ir.Package) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s dir=%q doc=%q\n", p.Name, p.Dir, p.Doc)
	for _, imp := range p.Imports {
		fmt.Fprintf(&b, "import %s dir=%q\n", imp.Name, imp.Dir)
		for _, e := range imp.Emits {
			fmt.Fprintf(&b, "  %s\n", emitText(e))
		}
	}
	for _, e := range p.Emits {
		fmt.Fprintf(&b, "emit %s\n", emitText(e))
	}
	for _, t := range p.Types {
		dumpType(&b, t)
	}
	for _, c := range p.Consts {
		fmt.Fprintf(&b, "const %s %s = %s%s\n", c.Name, typeText(&c.Type), valueText(c.V), namesText(c.Go, c.Cpp, c.TS))
	}
	for _, v := range p.Values {
		fmt.Fprintf(&b, "value %s %s schema=%s reload=%v ids=%v%s\n  = %s\n", v.Name, typeText(&v.Type), v.Schema, v.Reload, v.IDs,
			namesText(v.Go, v.Cpp, v.TS), valueText(v.V))
	}
	for _, fn := range p.Fns {
		dumpFn(&b, "", fn)
	}
	for _, d := range p.Defines {
		fmt.Fprintf(&b, "defines %s.%s %v %v\n", d.Pkg, d.Value, d.Names, d.Values)
	}
	return b.String()
}

func emitText(e *ir.Emit) string {
	values := "-"
	if e.Values != nil {
		values = fmt.Sprintf("%q", e.Values)
	}
	return fmt.Sprintf("%s out=%q dir=%q file=%q import=%q mode=%s values=%s package=%q namespace=%q",
		targetNames[e.Target], e.Out, e.Dir, e.FileName, e.GoImport, modeNames[e.Mode], values, e.GoPackage, e.Namespace)
}

func namesText(g, c, t ir.NameOptions) string {
	if g.Name == "" && c.Name == "" && t.Name == "" {
		return ""
	}
	return fmt.Sprintf(" names=%q/%q/%q", g.Name, c.Name, t.Name)
}

func dumpType(b *strings.Builder, t ir.Type) {
	switch x := t.(type) {
	case *ir.Record:
		fmt.Fprintf(b, "record %s params=%d doc=%q cpp=%+v%s\n", x.QName(), x.Params, x.Doc, x.Cpp, namesText(x.Go, ir.NameOptions{}, x.TS))
		dumpBody(b, "  ", x.Fields, x.Methods)
	case *ir.Enum:
		codes := "-"
		if x.Codes != nil {
			codes = typeText(x.Codes)
		}
		fmt.Fprintf(b, "enum %s codes=%s jsonCodes=%v ordered=%v defines=%q doc=%q%s\n", x.QName(), codes, x.JSONCodes, x.Ordered, x.CppDefines, x.Doc, namesText(x.Go, x.Cpp, x.TS))
		for _, m := range x.Members {
			fmt.Fprintf(b, "  member %s wire=%q index=%d code=%d retired=%v deprecated=%v doc=%q%s\n", m.Name, m.Wire, m.Index, m.Code, m.Retired, m.Deprecated, m.Doc, namesText(m.Go, m.Cpp, m.TS))
		}
	case *ir.Variant:
		fmt.Fprintf(b, "variant %s tag=%q doc=%q%s\n", x.QName(), x.Tag, x.Doc, namesText(x.Go, x.Cpp, x.TS))
		for _, c := range x.Cases {
			fmt.Fprintf(b, "  case %s wire=%q retired=%v cpp=%+v doc=%q%s\n", c.Name, c.Wire, c.Retired, c.Cpp, c.Doc, namesText(c.Go, ir.NameOptions{}, c.TS))
			dumpBody(b, "    ", c.Fields, c.Methods)
		}
	case *ir.Dependent:
		fmt.Fprintf(b, "dependent %s params=%d disc=param%d%v %s byMember=%v doc=%q\n", x.QName(), x.Params, x.DiscParam, x.DiscPath, typeText(x.Disc), x.ByMember, x.Doc)
		for _, br := range x.Branches {
			fmt.Fprintf(b, "  branch %s members=%v %s\n", br.Name, br.Members, typeText(&br.Type))
		}
	}
}

func dumpBody(b *strings.Builder, indent string, fields []*ir.Field, fns []*ir.ExportFn) {
	for _, f := range fields {
		fmt.Fprintf(b, "%sfield %s wire=%v %s opt=%v none=%q unit=%s enc=%d inline=%v pairs=%v default=%s computed=%v deprecated=%v stable=%v input=%v cpp=%+v bigint=%v doc=%q%s\n",
			indent, f.Name, f.WirePath, typeText(&f.Type), f.Optional, f.NoneWire, f.Unit, f.Enc, f.Inline, f.Pairs, valueText(f.Default),
			f.Computed, f.Deprecated, f.Stable, f.Input, f.Cpp, f.BigInt, f.Doc, namesText(f.Go, ir.NameOptions{}, f.TS))
	}
	for _, fn := range fns {
		dumpFn(b, indent, fn)
	}
}

func dumpFn(b *strings.Builder, indent string, fn *ir.ExportFn) {
	params := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		params[i] = p.Name + ": " + typeText(&p.Type)
	}
	fmt.Fprintf(b, "%sfn %s(%s) -> %s kind=%s doc=%q%s\n", indent, fn.Name, strings.Join(params, ", "), typeText(&fn.Result), kindNames[fn.Kind], fn.Doc, namesText(fn.Go, fn.Cpp, fn.TS))
	if fn.Value != nil {
		fmt.Fprintf(b, "%s  = %s\n", indent, valueText(fn.Value))
	}
	dumpTable(b, indent+"  ", fn.Table)
	for _, in := range fn.Instances {
		fmt.Fprintf(b, "%s  instance %s = %s\n", indent, valueText(in.Recv), valueText(in.Result))
		dumpTable(b, indent+"    ", in.Table)
	}
}

func dumpTable(b *strings.Builder, indent string, t *ir.LookupTable) {
	if t == nil {
		return
	}
	for i, d := range t.Domains {
		fmt.Fprintf(b, "%sdomain %d: %s\n", indent, i, valueList(d))
	}
	fmt.Fprintf(b, "%scells: %s\n", indent, valueList(t.Cells))
}

func valueList(vs []value.Value) string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = valueText(v)
	}
	return "[" + strings.Join(out, ", ") + "]"
}

func valueText(v value.Value) string {
	if v == nil {
		return "-"
	}
	return v.CanonText()
}

// typeText is a TypeRef as text: kinds by name, named types qualified, refs with their target.
func typeText(t *ir.TypeRef) string {
	if t == nil {
		return "nil"
	}
	switch t.Kind {
	case types.Enum, types.Record, types.Variant, types.TypeApp:
		return t.Named.QName() + argsText(t.Args)
	case types.Case:
		return t.Named.QName() + "." + t.Case.Name
	case types.Ref:
		r := t.Ref
		return fmt.Sprintf("ref(%s coll=%d %s.%s%v keyed=%v local=%v)", typeText(t.Key), r.Coll, r.Pkg, r.Value, r.Path, r.Keyed, r.Local)
	case types.List, types.Table, types.Optional, types.Map, types.DepMap, types.LitUnion:
		return compositeText(t)
	}
	if name := (types.Basic{K: t.Kind, Bits: t.Bits, Signed: t.Signed}).String(); name != "" {
		return name
	}
	return fmt.Sprintf("kind%d", t.Kind)
}

var (
	targetNames    = []string{"go", "cpp", "ts", "json", "view"}
	modeNames      = []string{"-", "baked", "embedded", "data", "types"}
	kindNames      = []string{"precomputed", "lookup", "translated"}
	compositeNames = map[types.Kind]string{
		types.List: "list", types.Table: "table", types.Optional: "opt",
		types.Map: "map", types.DepMap: "depmap", types.LitUnion: "union",
	}
)

func compositeText(t *ir.TypeRef) string {
	s := compositeNames[t.Kind] + "("
	if t.Key != nil {
		s += typeText(t.Key) + ", "
	}
	s += typeText(t.Elem)
	if t.KeyedBy != nil {
		s += fmt.Sprintf(" keyed %s%v", t.KeyedBy.Name, t.KeyedBy.WirePath)
	}
	if len(t.Literals) > 0 {
		s += fmt.Sprintf(" %q", t.Literals)
	}
	return s + ")"
}

func argsText(args []*ir.Source) string {
	if len(args) == 0 {
		return ""
	}
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = fmt.Sprintf("%d:%d%v", a.From, a.Param, a.WirePath)
	}
	return "<" + strings.Join(out, ",") + ">"
}
