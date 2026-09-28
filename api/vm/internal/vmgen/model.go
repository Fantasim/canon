package main

import (
	"fmt"
	"go/token"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// goType is a generated Go type.
type goType struct {
	kind typeKind
	elem *goType    // tSlice, tMap
	def  *structDef // tStruct
}

func (t *goType) String() string {
	if t.def != nil {
		return t.def.name
	}
	if t.elem != nil {
		return goNames[t.kind] + t.elem.String()
	}
	return goNames[t.kind]
}

// typed is a Go type, and whether its zero value is also a value the schema admits: an
// optional member of such a type needs a pointer to tell absent from present.
type typed struct {
	t      *goType
	zeroOK bool
	via    string // the union branch definition a $ref named, when its union's struct stands in
}

type structDef struct {
	name   string
	doc    string
	fields []*field
}

type field struct {
	json, name string
	t          *goType
	always     bool     // required in every branch: never omitted (VIEWMODEL.md J3)
	zeroOK     bool     // the zero value is admitted
	kinds      []string // the tags of the branches holding it, when not all do
	via        string   // see typed.via
}

// gen turns the schema into structs, in the order a depth-first walk from the root meets them.
type gen struct {
	defs    *node
	names   []nameOverride
	owner   map[string]string     // branch definition -> the union definition it belongs to
	byDef   map[string]*structDef // definition -> its struct
	byShape map[string]*structDef // canonical text of an inline object -> its struct
	taken   map[string]bool       // Go type names
	used    map[string]bool       // override sites met
	structs []*structDef
}

// generate is vm.gen.go for the schema data, the inline objects named by names.
func generate(data []byte, names []nameOverride) ([]byte, error) {
	root, err := parseSchema(data)
	if err != nil {
		return nil, err
	}
	g, err := newGen(root, names)
	if err != nil {
		return nil, err
	}
	sd, err := g.newStruct(rootName, docRoot)
	if err != nil {
		return nil, err
	}
	if err := g.flatten(sd, []*node{root}); err != nil {
		return nil, err
	}
	if err := g.finish(); err != nil {
		return nil, err
	}
	return emit(g.structs)
}

func newGen(root *node, names []nameOverride) (*gen, error) {
	g := &gen{
		defs: root.get(kwDefs), names: names, owner: map[string]string{},
		byDef: map[string]*structDef{}, byShape: map[string]*structDef{},
		taken: map[string]bool{}, used: map[string]bool{},
	}
	if err := g.checkDefs(); err != nil {
		return nil, err
	}
	for i, name := range g.defKeys() {
		refs := unionRefs(g.defs.vals[i])
		if refs == nil || !g.objectish(g.defs.vals[i]) {
			continue
		}
		for _, r := range refs {
			if _, dup := g.owner[r]; dup {
				return nil, fmt.Errorf(fmtAt, errOwner, r)
			}
			g.owner[r] = name
		}
	}
	return g, nil
}

func (g *gen) defKeys() []string {
	if g.defs == nil {
		return nil
	}
	return g.defs.keys
}

// unionRefs lists the definitions a oneOf made only of $ref branches names; nil otherwise.
func unionRefs(n *node) []string {
	branches := n.get(kwOneOf)
	if branches == nil {
		return nil
	}
	out := make([]string, 0, len(branches.vals))
	for _, b := range branches.vals {
		name, ok := strings.CutPrefix(b.get(kwRef).textOr(), defsPrefix)
		if !ok {
			return nil
		}
		out = append(out, name)
	}
	return out
}

// resolve is the definition a $ref names (nil when there is none).
func (g *gen) resolve(ref string) *node {
	name, ok := strings.CutPrefix(ref, defsPrefix)
	if !ok {
		return nil
	}
	return g.defs.get(name)
}

// objectish reports a node that becomes a struct: an object with members, or a oneOf of such.
func (g *gen) objectish(n *node) bool {
	if ref := n.get(kwRef); ref != nil {
		return g.objectish(g.resolve(ref.text))
	}
	if n.has(kwProperties) {
		return true
	}
	branches := n.get(kwOneOf)
	return branches != nil && !slices.ContainsFunc(branches.vals, func(b *node) bool { return !g.objectish(b) })
}

// branches are the object schemas a struct is made of: n itself, or its oneOf branches.
func (g *gen) branches(n *node, site string) ([]*node, error) {
	if n.has(kwProperties) {
		return []*node{n}, nil
	}
	if err := checkKeys(n, unionKeywords, site); err != nil {
		return nil, err
	}
	var out []*node
	for _, b := range n.get(kwOneOf).valsOr() {
		if ref := b.get(kwRef); ref != nil {
			b = g.resolve(ref.text)
		}
		out = append(out, b)
	}
	if out == nil {
		return nil, fmt.Errorf(fmtAt, errSchema, site)
	}
	return out, nil
}

// newStruct reserves a Go type name and appends its struct to the output.
func (g *gen) newStruct(name, doc string) (*structDef, error) {
	if g.taken[name] || !token.IsIdentifier(name) {
		return nil, fmt.Errorf(fmtAt, errName, name)
	}
	sd := &structDef{name: name, doc: doc}
	g.taken[name] = true
	g.structs = append(g.structs, sd)
	return sd, nil
}

// finish refuses an object definition no member reaches and an override no site used.
func (g *gen) finish() error {
	for i, name := range g.defKeys() {
		if _, special := specialDefs[name]; special || !g.objectish(g.defs.vals[i]) {
			continue
		}
		if o, ok := g.owner[name]; ok {
			name = o
		}
		if g.byDef[name] == nil {
			return fmt.Errorf(fmtAt, errUnreached, name)
		}
	}
	for _, o := range g.names {
		if !g.used[o.site] {
			return fmt.Errorf(fmtAt, errStaleName, o.site)
		}
	}
	return nil
}

// exported is the Go name of a member or definition name.
func exported(s string) (string, error) {
	s = strings.TrimPrefix(s, sigil)
	if v, ok := initialisms[s]; ok {
		return v, nil
	}
	r, size := utf8.DecodeRuneInString(s)
	out := string(unicode.ToUpper(r)) + s[size:]
	if !token.IsIdentifier(out) {
		return "", fmt.Errorf(fmtAt, errName, s)
	}
	return out, nil
}

// stringZeroOK reports whether "" satisfies a string schema.
func stringZeroOK(n *node) (bool, error) {
	if m := n.get(kwMinLength); m != nil && m.text != zeroNumber {
		return false, nil
	}
	p := n.get(kwPattern)
	if p == nil {
		return true, nil
	}
	re, err := regexp.Compile(p.text)
	if err != nil {
		return false, fmt.Errorf("pattern: %w", err)
	}
	return re.MatchString(""), nil
}

// intZeroOK reports whether 0 satisfies an integer schema.
func intZeroOK(n *node) bool {
	bound := func(kw string) float64 {
		v, err := strconv.ParseFloat(n.get(kw).textOr(), 64)
		if err != nil {
			return 0
		}
		return v
	}
	return bound(kwMinimum) <= 0 && bound(kwMaximum) >= 0
}
