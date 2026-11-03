package build_test

import (
	"fmt"
	"strings"
)

// The sizes of project (a), a wide record, a wide variant case and a table, and of project (b), an enum.
const (
	wideMembers     = 300
	wideCaseMembers = 100
	tableRows       = 10000
	codeMembers     = 5000
	shapeCount      = 14
)

// The modes of the budget packages, and the name of project (b)'s lookup fn.
const (
	modeTypes    = "types"
	modeData     = "data"
	modeBaked    = "baked"
	lookupFnName = "weight"
)

// The codes of the opened enum: sparse, and one member in retiredEvery retired.
const (
	codeStride   = 7
	codeOffset   = 3
	retiredEvery = 97
)

// emits writes the emit lines of a package for the targets and the mode; the TypeScript target is
// left out when ts is false.
func emits(pkg, mode string, ts bool, values ...string) string {
	var b strings.Builder
	if mode == modeData {
		fmt.Fprintf(&b, "emit json { out: \"@out/%s/json/\", values: [%s] }\n", pkg, strings.Join(values, ", "))
	}
	fmt.Fprintf(&b, "emit go { out: \"@out/%s/go\", package: \"%s\", mode: %s }\n", pkg, pkg, mode)
	fmt.Fprintf(&b, "emit cpp { out: \"@out/%s/cpp\", namespace: \"%s\", mode: %s }\n", pkg, pkg, mode)
	if ts {
		fmt.Fprintf(&b, "emit ts { out: \"@out/%s/%s.ts\", mode: %s }\n", pkg, pkg, mode)
	}
	return b.String()
}

// wideField is member i of Wide: shapeCount type shapes in turn, each with a default.
func wideField(i int) string {
	name := fmt.Sprintf("f%03d", i)
	shapes := [shapeCount]string{
		fmt.Sprintf("Int = %d", i),
		"Float = 1.5",
		"Bool = false",
		"String = \"s\"",
		"Duration = 5s",
		"Color = red",
		"Int?",
		"[Int] = [1, 2]",
		"{String: Int} = {\"a\": 1}",
		"Inner = { x: 1, y: \"a\" }",
		"Kind = nothing",
		"ref rows?",
		"String(1..) = \"s\"",
		"[Inner] = [{ x: 2, y: \"b\" }]",
	}
	return fmt.Sprintf("  /// Member %d.\n  %s: %s\n", i, name, shapes[i%shapeCount])
}

// wideSource is package pkg of project (a): Wide, a variant with a wide case, and a stable table of
// tableRows rows with a nested record and a @stable field, emitted in mode for the three targets.
func wideSource(pkg, mode string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "/// Project (a) in %s mode.\npackage %s\n\n", mode, pkg)
	b.WriteString("/// Colours.\nenum Color { red, green, blue }\n\n")
	b.WriteString("/// A nested record.\nrecord Inner {\n  /// X.\n  x: Int = 0\n  /// Y.\n  y: String = \"\"\n}\n\n")
	b.WriteString("/// A variant with a wide case.\nvariant Kind @json(tag: \"type\") {\n  /// Little.\n  little {\n    /// N.\n    n: Int = 0\n  }\n  /// Wide.\n  wide {\n")
	for i := range wideCaseMembers {
		fmt.Fprintf(&b, "    /// Member %d.\n    w%03d: Int = %d\n", i, i, i)
	}
	b.WriteString("  }\n  /// Nothing.\n  nothing\n}\n\n")
	b.WriteString("/// A wide record.\nrecord Wide {\n")
	for i := range wideMembers {
		b.WriteString(wideField(i))
	}
	b.WriteString("}\n\n/// A row.\nrecord Row {\n  /// N, the field FindByN looks rows up by.\n  n: Int = 0 @stable\n  /// Name.\n  name: String = \"\"\n  /// Switch.\n  on: Bool = false\n  /// Tags.\n  tags: [String] = []\n  /// Nested.\n  inner: Inner = {}\n}\n\n")
	b.WriteString("/// The wide value.\nlet wide: Wide = {}\n\n/// The rows.\nlet rows: stable table Row = {\n")
	for i := range tableRows {
		fmt.Fprintf(&b, "  R%05d { n: %d, name: \"row %d\", on: true, tags: [\"t\", \"u%d\"], inner: { x: %d, y: \"y%d\" } }\n", i, i, i, i, i, i)
	}
	b.WriteString("}\n\n")
	b.WriteString(emits(pkg, mode, true, "wide", "rows"))
	return b.String()
}

// codesSource is project (b): a @codes enum, a lookup fn, a list and an input field (which allows no TS emit, §5.12).
func codesSource(pkg, mode string, input, lookup bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "/// Project (b) in %s mode.\npackage %s\n\n", mode, pkg)
	b.WriteString("/// Codes.\nenum Code @codes(UInt16) {\n")
	for i := range codeMembers {
		fmt.Fprintf(&b, "  M%04d = %d\n", i, i+1)
	}
	b.WriteString("}\n\n")
	if lookup {
		fmt.Fprintf(&b, "/// The weight of a code.\nexport fn %s(c: Code) -> Int {\n  return match c {\n", lookupFnName)
		for i := range codeMembers {
			fmt.Fprintf(&b, "    M%04d => %d\n", i, i)
		}
		b.WriteString("  }\n}\n\n")
	}
	b.WriteString("/// A holder.\nrecord Holder {\n  /// A list of codes.\n  list: [Code] = [M0000, M0001]\n")
	if input {
		b.WriteString("  /// The code the server runs with.\n  boot: input Code from env \"BUDGET_CODE\"\n")
	}
	b.WriteString("}\n\n/// The holder.\nlet holder: Holder = {}\n\n")
	b.WriteString(emits(pkg, mode, !input, "holder"))
	return b.String()
}

// openCodesSource is project (b) with an opened enum: only types mode opens one, which has no lookup fn or input field.
func openCodesSource(pkg string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "/// Project (b) with an opened enum.\npackage %s\n\n/// Codes.\nenum Code @codes(Int32) {\n", pkg)
	for i := range codeMembers {
		retired := ""
		if i%retiredEvery == 0 {
			retired = "retired "
		}
		fmt.Fprintf(&b, "  %sM%04d = %d\n", retired, i, i*codeStride+codeOffset)
	}
	b.WriteString("}\n\n/// A holder.\nrecord Holder {\n  /// A list of codes.\n  list: [Code] = [M0001, M0002]\n  /// Weights by code.\n  weights: {Code: Int} = {}\n  /// A code or none.\n  maybe: Code?\n}\n\n")
	for _, target := range []string{"go", "cpp"} {
		fmt.Fprintf(&b, "emit %s { out: \"@out/%s/%s\", %s: \"%s\", mode: %s, open: [Code] }\n", target, pkg, target, map[string]string{"go": "package", "cpp": "namespace"}[target], pkg, modeTypes)
	}
	fmt.Fprintf(&b, "emit ts { out: \"@out/%s/%s.ts\", mode: %s, open: [Code] }\n", pkg, pkg, modeTypes)
	return b.String()
}

// edgePkg is the package of the boundary fixtures.
const edgePkg = "edge"

// buildEdgeSource is a baked package whose build function holds more lines for each of fields: W
// has that many required members, and there are a value of W and a table of one row.
func buildEdgeSource(fields int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "/// Boundary.\npackage %s\n\n/// A row.\nrecord R {\n  /// N.\n  n: Int\n}\n\n/// W.\nrecord W {\n", edgePkg)
	for i := range fields {
		fmt.Fprintf(&b, "  /// F.\n  f%04d: Int\n", i)
	}
	b.WriteString("}\n\n/// V.\nlet w: W = {\n")
	for i := range fields {
		fmt.Fprintf(&b, "  f%04d: %d\n", i, i)
	}
	b.WriteString("}\n\n/// T.\nlet t: table R = {\n  K0 { n: 0 }\n}\n\n")
	b.WriteString(emits(edgePkg, modeBaked, false))
	return b.String()
}

// decodeEdgeSource is a types package of one record W whose decoder holds more lines for each
// defaulted member than for each required one.
func decodeEdgeSource(defaulted, required int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "/// Boundary.\npackage %s\n\n/// W.\nrecord W {\n", edgePkg)
	for i := range defaulted {
		fmt.Fprintf(&b, "  /// D.\n  d%04d: Int = %d\n", i, i)
	}
	for i := range required {
		fmt.Fprintf(&b, "  /// R.\n  r%04d: Int\n", i)
	}
	b.WriteString("}\n\n")
	b.WriteString(emits(edgePkg, modeTypes, false))
	return b.String()
}
