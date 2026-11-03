package ir_test

import (
	"fmt"
	"strings"
)

// mxMode is where a chain of the ref matrix is generated (TestCppRefMatrix: CODEGEN.md §5.3, §5.8, §5.9, DECISIONS 387, 456): baked, data with @reload, types, or read by another package's data and types emits.
type mxMode string

const (
	mxBaked  mxMode = "bk"
	mxData   mxMode = "da"
	mxStatic mxMode = "ds" // data without @reload: a dependent type is decided through a ref only into a let that is not @reload
	mxTypes  mxMode = "ty"
	mxReader mxMode = "rd"
)

var mxModes = []mxMode{mxBaked, mxData, mxStatic, mxTypes, mxReader}

// mxLeaf is the key a chain of refs ends in: a type, or a table whose ids are the keys.
type mxLeaf struct {
	code, decl, typ string
	lits            [2]string // the two keys the rows and the values name, each written as a key of the leaf
	table, own      bool      // a table's ids; its own: held as an id enum in a baked package only
	foreign         bool      // a table of another package
}

const (
	mxLib       = "mxlib"
	mxShape     = "enum Shape { count, place, none_ }"
	mxTableName = "tbl"
)

var mxLeaves = []mxLeaf{
	{code: "ew", decl: "enum Tone { first, second = \"z2\", third }", typ: "Tone", lits: [2]string{"second", "third"}},
	{code: "ec", decl: "enum Rank @codes(UInt8) { zulu = 4, alpha = 9, mid = 1 }", typ: "Rank", lits: [2]string{"alpha", "zulu"}},
	{code: "st", typ: "String", lits: [2]string{`"q"`, `"p"`}},
	{code: "in", typ: "Int", lits: [2]string{"20", "5"}},
	{code: "ot", lits: [2]string{"sword", "shield"}, table: true, own: true},
	{code: "ft", lits: [2]string{"sword", "shield"}, table: true, foreign: true},
}

// mxPos is a position a ref is read at: its code, and its fields and their values in the Holder.
type mxPos struct {
	code           string
	fields, values []string // %[1]s is the ref's target; a value's %[1]s and %[2]s are the two keys
}

var mxPositions = []mxPos{
	{code: "f", fields: []string{"f: ref %[1]s"}, values: []string{"f: %[1]s"}},
	{code: "l", fields: []string{"fs: [ref %[1]s]"}, values: []string{"fs: [%[2]s]"}},
	{code: "m", fields: []string{"fm: {ref %[1]s: Int}"}, values: []string{"fm: { %[1]s: 1 }"}},
	{code: "u", fields: []string{"fu: ref %[1]s | \"none\"", "fus: [ref %[1]s | \"none\"]"}, values: []string{"fu: %[1]s", "fus: [\"none\", %[2]s]"}},
	{code: "d", fields: []string{"ev: ref %[1]s", "param: Param(ev.shape)? @json(none: 0)"}, values: []string{"ev: %[1]s", "param: 3"}},
}

// mxCase is one chain: a leaf kind, a depth of refs, a mode.
type mxCase struct {
	leaf  mxLeaf
	depth int
	mode  mxMode
	tag   string // told apart from the same case's other packages: a probe's position
}

func (c mxCase) name() string { return fmt.Sprintf("mx%s%d%s%s", c.leaf.code, c.depth, c.mode, c.tag) }

// legal reports the combinations Canon has: an own table's ids are an enum in a baked package only.
func (c mxCase) legal() bool { return !c.leaf.own || c.mode == mxBaked || c.mode == mxReader }

// lists is the number of keyed lists the chain holds: one per ref, less the ref into the table.
func (c mxCase) lists() int {
	if c.leaf.table {
		return c.depth - 1
	}
	return c.depth
}

// table is the name of the table a chain ends in.
func (c mxCase) table() string {
	if c.leaf.foreign {
		return "items"
	}
	return mxTableName
}

// target is what a ref at the chain's depth points at.
func (c mxCase) target() string {
	switch {
	case c.leaf.table && c.depth == 1:
		return c.table()
	case c.leaf.table:
		return fmt.Sprintf("l%d", c.depth-1)
	}
	return fmt.Sprintf("l%d", c.depth)
}

// mxDoc is a declaration's doc comment, which every declaration has.
func mxDoc(s string) string { return "/// " + s + ".\n" }

// declarations are the leaf, Shape and the chain's records and lists, each doc-commented; reload marks every list @reload.
func (c mxCase) declarations(reload bool) string {
	var b strings.Builder
	if c.leaf.foreign {
		fmt.Fprintf(&b, "import %s { items, Shape }\n\n", mxLib)
	} else {
		b.WriteString(mxDoc("Shape") + mxShape + "\n\n")
	}
	if c.leaf.decl != "" {
		b.WriteString(mxDoc("Leaf") + c.leaf.decl + "\n\n")
	}
	if c.leaf.own {
		b.WriteString(mxDoc("T") + "record T {\n" + mxDoc("Shape") + "  shape: Shape\n}\n\n")
		b.WriteString(mxDoc("The table") + "let " + mxTableName + ": table T = {\n  sword { shape: count }\n  shield { shape: place }\n}\n\n")
	}
	for i := 1; i <= c.lists(); i++ {
		key := c.leaf.typ
		if i > 1 {
			key = fmt.Sprintf("ref l%d", i-1)
		} else if c.leaf.table {
			key = "ref " + c.table()
		}
		b.WriteString(fmt.Sprintf("%srecord R%d {\n%s  k: %s\n%s  shape: Shape\n}\n\n", mxDoc("Row"), i, mxDoc("Key"), key, mxDoc("Shape")))
		rows := fmt.Sprintf("{ k: %s, shape: count }, { k: %s, shape: place }", c.leaf.lits[0], c.leaf.lits[1])
		b.WriteString(mxDoc("List") + mxReloadLine(reload) + fmt.Sprintf("let l%d: [R%d] keyed by k = [ %s ]\n\n", i, i, rows))
	}
	return b.String()
}

// mxReloadLine is @reload, on its own line, or nothing.
func mxReloadLine(reload bool) string {
	if reload {
		return "@reload\n"
	}
	return ""
}

// holder is the record of the positions, its dependent type and, with a value, its let.
func (c mxCase) holder(pos []mxPos) string {
	var b strings.Builder
	var fields, values []string
	for _, p := range pos {
		fields = append(fields, p.fields...)
		values = append(values, p.values...)
		if p.code == "d" {
			b.WriteString(mxDoc("A parameter, by shape") + "type Param(s: Shape) = match s {\n  count => Int(0..)\n  place => String\n  none_ => Never\n}\n\n")
		}
	}
	b.WriteString(mxDoc("H") + "record H {\n")
	for _, f := range fields {
		b.WriteString(mxDoc("A field") + "  " + mxFill(f, c.target(), "") + "\n")
	}
	b.WriteString("}\n\n")
	return b.String()
}

// value is the let of a holder, naming the keys at each position.
func (c mxCase) value(pos []mxPos, reload bool) string {
	var parts []string
	for _, p := range pos {
		for _, v := range p.values {
			parts = append(parts, mxFill(v, c.leaf.lits[0], c.leaf.lits[1]))
		}
	}
	return mxDoc("The holder") + mxReloadLine(reload) + "let h: H = { " + strings.Join(parts, ", ") + " }\n\n"
}

// emit is the emit lines of a package in mode.
func mxEmit(pkg string, mode string, json bool) string {
	out := fmt.Sprintf("emit cpp { out: \"@features/%[1]s\", namespace: \"%[1]s\", mode: %[2]s }\n", pkg, mode)
	if json {
		out += "emit json { out: \"out/\" }\n"
	}
	return out
}

// mxFill writes the first and the second argument where a template holds %[1]s and %[2]s.
func mxFill(template, first, second string) string {
	return strings.NewReplacer("%[1]s", first, "%[2]s", second).Replace(template)
}

// mxFile is one .canon file of the matrix, by its package.
type mxFile struct{ pkg, text string }

// files are the packages of the case, positions given: one for a baked, data or types mode; for the reader mode the owner (baked), a data reader holding a value and a types reader.
func (c mxCase) files(pos []mxPos) []mxFile {
	head := func(pkg string) string { return mxDoc("Package") + "package " + pkg + "\n\n" }
	if c.mode != mxReader {
		reload := c.mode == mxData
		text := head(c.name()) + c.declarations(reload) + c.holder(pos) + c.value(pos, reload)
		mode := map[mxMode]string{mxBaked: "baked", mxData: "data", mxStatic: "data", mxTypes: "types"}[c.mode]
		return []mxFile{{c.name(), text + mxEmit(c.name(), mode, c.mode == mxData || c.mode == mxStatic)}}
	}
	owner, data, typed := c.name()+"a", c.name()+"b", c.name()+"t"
	return []mxFile{
		{owner, head(owner) + c.declarations(false) + c.holder(pos) + mxEmit(owner, "baked", false)},
		{data, head(data) + fmt.Sprintf("import %s { H }\n\n", owner) + c.value(pos, false) + mxEmit(data, "data", true)},
		{typed, head(typed) + fmt.Sprintf("import %s { H }\n\n%srecord W {\n%s  h: H\n}\n\n", owner, mxDoc("W"), mxDoc("The holder")) + mxEmit(typed, "types", false)},
	}
}

// mxLibFile is the library of the foreign leaf: Shape and a baked table of items.
func mxLibFile() mxFile {
	text := mxDoc("Package") + "package " + mxLib + "\n\n" + mxDoc("Shape") + mxShape + "\n\n" +
		mxDoc("An item") + "record Item {\n" + mxDoc("Shape") + "  shape: Shape\n}\n\n" +
		mxDoc("The items") + "let items: table Item = {\n  sword { shape: count }\n  shield { shape: place }\n}\n\n" +
		mxEmit(mxLib, "baked", false)
	return mxFile{mxLib, text}
}
