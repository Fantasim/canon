package cppgen

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

var (
	//go:embed runtime/canon_runtime.h.txt
	runtimeText string
	//go:embed runtime/canon_runtime_json.h.txt
	runtimeJSONText string
)

// gen is one run of the generator over a package and one of its cpp emits.
type gen struct {
	p            *ir.Package
	emit         *ir.Emit
	pl           *ir.CppNamePlan // the names ir plans (CODEGEN.md §3.3–§3.5)
	err          error
	at           string // the Canon item being written, for messages
	last         string // the last segment of the package: the file stem
	values       []*ir.Value
	entries      map[*ir.Record]bool      // records that are entries of an emitted table
	loaders      map[*ir.Record]*ir.Value // a record's static Load reads this non-@reload value
	boxed        map[*ir.Field]bool       // optional fields held through a std::unique_ptr
	imported     map[string]bool          // the headers of the imported packages the code uses
	pairsFriends map[*ir.Record][]string  // the classes decoding a record from pairs: slots
	foreignEnums []*ir.Enum               // imported enums the code names, in first-use order
	holders      map[any][]*ir.Value      // per class, the emitted values holding it
	slots        map[any][]resolved       // per class, the refs resolved at load
	walking      class                    // the class whose Resolve is being written (its map keys are checked)
	walkSnap     bool                     // that Resolve finds entries in the snapshot, else its holder
	classes      []class                  // records, cases, variants, dependent types, topologically sorted (§2.7)
	inputs       []inputField             // input fields, declaration order (§5.12)
	inputNames   ir.CppInputs             // LoadInputs, the slots' namespace and flag, the helpers (§7.7)
	top          *scope
	pkgFns       []*ir.ExportFn          // package-level translated fns
	methods      []*method               // translated methods, declaration order
	bk           *bakedIndex             // a baked emit's entries and precomputed results (CODEGEN.md §5.9)
	dst          string                  // what a decoder fills a member through: out., or "" in a reader's locals (CODEGEN.md §2.8)
	view         string                  // the package whose class layout storage describes; "" for this one (§5.14)
	qualify      bool                    // this package's names written from the global namespace, as detail::<P>Make does (§5.14)
	rowFriends   map[*ir.Record][]string // per row of another package's record, the classes whose decoders fill it (§5.9)
	h, c         writer
}

// method is a translated method with the class that owns it and that class's fields.
type method struct {
	fn     *ir.ExportFn
	class  class
	fields []*ir.Field
}

// Generate is the C++ generator (ir.Generator): every file of CODEGEN.md §2.3 for one cpp emit; p is narrowed by ir.CopyOf.
func Generate(p *ir.Package, e *ir.Emit) ([]ir.File, error) {
	if p == nil || e == nil || e.Target != ir.TargetCpp {
		return nil, ErrTarget
	}
	if e.Mode == ir.ModeNone || e.Mode == ir.ModeEmbedded {
		// stage E refuses both first: embedded is E8019 `unbuilt` (DECISIONS 320), none is check's E8009.
		return nil, fmt.Errorf("%w: mode %s of %s", ErrMalformed, modeText(e.Mode), e.Out)
	}
	if e.Namespace == "" || p.Dir == "" || p.Name == "" {
		return nil, fmt.Errorf("%w: cpp emit %s without a namespace or a package directory", ErrMalformed, e.Out)
	}
	g := newGen(p, e)
	g.plan()
	if g.err != nil {
		return nil, g.err
	}
	files := g.files()
	if g.err != nil {
		return nil, g.err
	}
	return files, nil
}

func newGen(p *ir.Package, e *ir.Emit) *gen {
	segs := strings.Split(p.Name, qnameSep)
	last := segs[len(segs)-1]
	return &gen{
		p: p, emit: e, at: p.Name, last: last,
		entries: map[*ir.Record]bool{}, loaders: map[*ir.Record]*ir.Value{}, imported: map[string]bool{},
		pairsFriends: map[*ir.Record][]string{}, holders: map[any][]*ir.Value{}, slots: map[any][]resolved{},
		rowFriends: map[*ir.Record][]string{}, top: newScope(e.Namespace), dst: outPrefix,
	}
}

// fail keeps the first error: generation goes on, so every function stays linear.
func (g *gen) fail(err error) {
	if g.err == nil && err != nil {
		g.err = err
	}
}

// refuseKind is a kind gen/cpp has no form for at this position: stage E refuses every one first (E8019 for an optional element, a dependent map, a case or a record constant; E8012 for Never, a Range, a function, a pair and a define record; a variant's Kind cannot be written, TYPES.md §2), so meeting one is malformed IR (DECISIONS 320).
func (g *gen) refuseKind(k types.Kind, where string) {
	g.malformed(kindText(k), where)
}

// declare adds a name of the emit's namespace.
func (g *gen) declare(name, origin string) {
	g.fail(g.top.add(name, origin))
}

// enter makes origin the item being written until the returned func restores the previous one.
func (g *gen) enter(origin string) (leave func()) {
	prev := g.at
	g.at = origin
	return func() { g.at = prev }
}

// plan refuses what this generator cannot emit, then indexes values, classes and fns.
func (g *gen) plan() {
	g.validate()
	if g.err != nil {
		return
	}
	g.pl = ir.PlanCppNames(g.p, g.emit)
	if probs := g.pl.Problems(); len(probs) > 0 {
		g.fail(fmt.Errorf("%w: %s (%s)", ErrMalformed, namePlanProblem, probs[0].Origin))
		return
	}
	if !g.types() {
		g.selectValues()
	}
	g.indexFns()
	g.boxes()
	if g.baked() {
		g.indexBaked()
	} else {
		g.pairsParents()
	}
	g.nestedRows()
	g.holdersOf()
	g.sortClasses()
	g.collectInputs()
	g.typesRefusals()
	g.declareNames()
}

// baked reports a baked emit: values built into the program, nothing decoded (CODEGEN.md §2.2, §5.9).
func (g *gen) baked() bool { return g.emit.Mode == ir.ModeBaked }

// types reports a types-mode emit: read-only types and their public decoders, no value (CODEGEN.md §2.2, §5.13).
func (g *gen) types() bool { return g.emit.Mode == ir.ModeTypes }

// files writes every output of the emit (CODEGEN.md §2.3).
func (g *gen) files() []ir.File {
	header := g.header()
	source := g.source()
	out := []ir.File{{Path: runtimeFile, Content: []byte(runtimeText)}}
	if !g.baked() {
		out = append(out, ir.File{Path: runtimeJSONFile, Content: []byte(runtimeJSONText)})
	}
	out = append(out, ir.File{Path: g.last + genHeaderSuffix, Content: header}, ir.File{Path: g.last + genSourceSuffix, Content: source})
	if g.translated() {
		out = append(out, ir.File{Path: g.last + conformanceSuffix, Content: g.conformance()})
	}
	if defines := g.definesFile(); defines != nil {
		out = append(out, ir.File{Path: g.last + definesSuffix, Content: defines})
	}
	return out
}

// translated reports a translated function: the package then has a conformance file.
func (g *gen) translated() bool { return len(g.pkgFns)+len(g.methods) > 0 }

// header is <last>.gen.h in §2.7's order after `#pragma once` (log-2026-09-24, Header guard).
func (g *gen) header() []byte {
	g.h = writer{}
	sections := []func(){
		g.constants, g.schemaConstants, g.enums, g.enumConstants, g.forwards,
		g.detailDecls, g.classDecls, g.makeStructDef, g.containers, g.accessors, g.foreignLoaderDecls, g.lateHooks,
		g.packageFns, g.snapshot, g.inputsDecl,
		g.conformanceDecl,
	}
	for _, s := range sections {
		s()
	}
	var out writer
	out.printf(markerFormat, g.p.Dir)
	out.line(pragmaOnce)
	out.blank()
	includes(&out, g.h.String(), headerGroups(g))
	g.namespaceBody(&out, g.emit.Namespace, g.h.String())
	return out.bytes()
}

// headerGroups are <nlohmann/json_fwd.hpp> when the header declares a decoder, then the runtime.
func headerGroups(g *gen) [][]string {
	last := append([]string{includeRuntime}, g.importIncludes()...)
	if len(g.classes) == 0 || g.baked() {
		return [][]string{last}
	}
	return [][]string{{includeJSONFwd}, last}
}

// source is <last>.gen.cpp: the helpers decoders call, decoders, the access struct, out-of-line members, then the runtime inputs (§2.7, §7.7).
func (g *gen) source() []byte {
	g.c = writer{}
	g.defineTables()
	if g.baked() {
		g.bakedAccess()
	} else {
		g.readerDecls()
		g.decoders()
		g.readers()
		g.accessStruct()
	}
	body := g.c.String()
	g.c = writer{}
	g.c.line(detailOpen)
	g.c.blank()
	g.c.write(body)
	g.c.line(detailClose)
	g.c.blank()
	g.outOfLine()
	g.loadInputs()
	var out writer
	out.printf(markerFormat, g.p.Dir)
	out.linef(0, includeQuotedFormat, g.last+genHeaderSuffix)
	out.blank()
	var groups [][]string
	if !g.baked() {
		groups = [][]string{{includeRuntimeJSON}}
	}
	includes(&out, g.c.String(), groups)
	g.namespaceBody(&out, g.emit.Namespace, g.c.String())
	return out.bytes()
}

// selectValues is the emit's values in declaration order; data mode takes tables, keyed lists and records (E8015), baked mode any value.
func (g *gen) selectValues() {
	for _, v := range g.p.Values {
		if g.emit.Values != nil && !slices.Contains(g.emit.Values, v.Name) {
			continue
		}
		leave := g.enter(v.Name)
		switch {
		case v.Type.Kind == types.Table:
			if rec, ok := v.Type.Elem.Named.(*ir.Record); ok {
				g.entries[rec] = true
			}
		case v.Type.Kind == types.List && v.Type.KeyedBy != nil:
		case g.baked():
		case v.Type.Kind == types.Record:
			g.recordLoader(v)
		default:
			g.malformed(dataValueKind, v.Name) // E8015
		}
		g.values = append(g.values, v)
		leave()
	}
}

// recordLoader gives a record value's type its static Load, the first value's (CODEGEN.md §5.9).
func (g *gen) recordLoader(v *ir.Value) {
	rec, ok := v.Type.Named.(*ir.Record)
	if !ok {
		g.fail(fmt.Errorf("%w: record value %s without its record", ErrMalformed, v.Name))
		return
	}
	if _, seen := g.loaders[rec]; !seen && !v.Reload && rec.Pkg == g.p.Name { // another package's: the free Load<V> (§5.9)
		g.loaders[rec] = v
	}
}

// dataFile is the file the package's emit json writes v to: <value>.json (WIRE.md §8.1, E8153).
func dataFile(v *ir.Value) string { return v.Name + ir.JSONExt }

func modeText(m ir.Mode) string {
	if int(m) < len(modeNames) {
		return modeNames[m]
	}
	return fmt.Sprint(m)
}
