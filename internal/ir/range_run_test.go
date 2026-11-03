package ir_test

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/testkit/cxx"
	"github.com/fantasim/canonlang/internal/testkit/tsc"
)

const (
	rangeRunProject    = "testdata/e2e/rangerun/project.txtar"
	rangeRunDir        = "testdata/e2e/rangerun"
	rangeRunBaseFile   = "base.json"
	rangeRunCasesFile  = "cases.txt"
	rangeRunDataFile   = "data.txt"
	rangeRunWidthFile  = "width_forms.txt"
	rangeRunSep        = "~~"
	rangeRunComment    = "#"
	rangeRunNewline    = `\n`
	rangeRunTextOnly   = "text"
	rangeRunTSDecode   = "tsdecode"
	rangeRunRefused    = "refused"
	rangeRunCasesDir   = "cases"
	rangeRunDataExt    = ".data"
	rangeRunSchemaFile = "app/out/app/probe.json"
	rangeRunEmitted    = "app/out/app/"
	rangeRunLoaderPkg  = "loader"
	rangeRunLoaderIn   = "value.json"
	rangeRunLoaderOut  = "loader/out/value.json"
	// rangeRunLoader loads value.json as lib's Probe and writes the value `load` gives as a data file.
	rangeRunLoader     = "package loader\n\nimport lib { Probe }\n\n/// The probe under test.\nlet value: Probe = load(\"value.json\")\n\nemit json { out: \"out/\" }\n"
	rangeRunKindProbe  = "probe"
	rangeRunKindItems  = "items"
	rangeRunKindTags   = "tags"
	rangeRunKindCaps   = "caps"
	rangeRunFileTag    = "-file"
	rangeRunKindSep    = ":"
	rangeRunOK         = "ok"
	rangeRunErrorPre   = "error "
	rangeRunDataTag    = "-data"
	rangeRunDecodeTag  = "-decode"
	rangeRunTSTag      = "ts-"
	rangeRunTagSep     = "-"
	rangeRunGoDir      = "probe"
	rangeRunGoRun      = "run"
	rangeRunOutDirFlag = "--outDir"
	rangeRunGoFile     = "main.go"
	rangeRunLibDir     = "libc"
	rangeRunLibSrc     = "lib.gen.cpp"
	rangeRunAppDir     = "appc"
	rangeRunAppSrc     = "app.gen.cpp"
	rangeRunLibTS      = "lib.ts"
	rangeRunAppTS      = "app.ts"
	rangeRunReaderSep  = "/"
	rangeRunLabelProbe = "Probe"
	rangeRunLabelCaps  = "CapsFile"
	rangeRunEnvelope   = "value"
	rangeRunTypesRoot  = "$"
	rangeRunPathSep    = "."
	rangeRunMsgSep     = ": "
	rangeRunWidth      = "<integer width>"
	rangeRunCppMain    = "probe.cpp"
	rangeRunCppBin     = "probe"
	rangeRunTSOut      = "js"
	rangeRunTSMain     = "probe.mjs"
	rangeRunTSPkg      = "package.json"
	rangeRunTSModule   = "{\"type\": \"module\"}\n"
	rangeRunGoMain     = "probe_go.txt"
	rangeRunCppSrc     = "probe_cpp.txt"
	rangeRunTSSrc      = "probe_ts.txt"
)

// rangeRunFirstAt is the one place a duplicate-id message names a path: it names it from the root of the file read, `$` in a types-mode decoder, `value` in a data file's envelope.
var rangeRunFirstAt = strings.NewReplacer("first at $.", "first at ", "first at value.", "first at ")

// The readers each target prints, by name; a name is followed by / and the compiler that built it, in C++ and TypeScript.
var (
	rangeRunGoReaders  = []string{"go-types", "go-data", "go-file"}
	rangeRunCppReaders = []string{"cpp-parse", "cpp-decode", "cpp-data", "cpp-file"}
	rangeRunTSReaders  = []string{"ts-parse", "ts-decode", "ts-data", "ts-file"}
)

// rangeRunWidthForms are the TypeScript decoders' forms of the one accepted difference between the targets' messages, ADR-0023 Decision 5, listed in width_forms.txt: the width of an integer. Go and C++ write it as `expected an integer from <lo> to <hi>`, or as the short `expected an integer` for Int's full range, and their readers are compared as written; a message of a TypeScript reader that has one of these forms is compared as rangeRunWidth, and so is the want, when it is compared with a TypeScript reader.
func rangeRunWidthForms(t *testing.T) []*regexp.Regexp {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(rangeRunDir, rangeRunWidthFile))
	if err != nil {
		t.Fatal(err)
	}
	var forms []*regexp.Regexp
	for _, line := range strings.Split(string(raw), "\n") {
		if line != "" && !strings.HasPrefix(line, rangeRunComment) {
			forms = append(forms, regexp.MustCompile(line))
		}
	}
	return forms
}

// rangeRunCase is one file every reader reads, what they must say of it (want; dataWant, where the data loaders say otherwise, WIRE.md §5.1: a unit Duration is an integer count of the unit in a data file and any whole number of milliseconds in a source) and whether `load` accepts it (a probe case only).
type rangeRunCase struct {
	name, kind, text, want, dataWant string
	loads, textOnly, tsDecodeOnly    bool
}

// rangeRunRows reads the cases of name in rangeRunDir: one row of trimmed fields per line, the comments and blank lines skipped.
func rangeRunRows(t *testing.T, name string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(rangeRunDir, name))
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]string
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" || strings.HasPrefix(line, rangeRunComment) {
			continue
		}
		fields := strings.Split(line, rangeRunSep)
		for i, f := range fields {
			fields[i] = strings.TrimSuffix(strings.TrimPrefix(f, " "), " ")
		}
		rows = append(rows, fields)
	}
	return rows
}

// rangeRunEdit is text with from replaced by to once; from empty keeps text.
func rangeRunEdit(t *testing.T, name, text, from, to string) string {
	t.Helper()
	if from == "" {
		return text
	}
	if !strings.Contains(text, from) {
		t.Fatalf("%s: %q is not in the text", name, from)
	}
	return strings.Replace(text, from, to, 1)
}

// rangeRunProbeCases are cases.txt applied to base.json.
func rangeRunProbeCases(t *testing.T) []rangeRunCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(rangeRunDir, rangeRunBaseFile))
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSuffix(string(raw), "\n")
	var cases []rangeRunCase
	for _, f := range rangeRunRows(t, rangeRunCasesFile) {
		c := rangeRunCase{name: f[0], kind: rangeRunKindProbe, text: rangeRunEdit(t, f[0], base, f[1], f[2]), want: f[3]}
		flags := ""
		if len(f) > 4 {
			flags = f[4]
		}
		c.loads = c.want == rangeRunOK && !strings.Contains(flags, rangeRunRefused)
		c.textOnly = strings.Contains(flags, rangeRunTextOnly)
		c.tsDecodeOnly = strings.Contains(flags, rangeRunTSDecode)
		if len(f) > 5 {
			c.dataWant = f[5]
		}
		cases = append(cases, c)
	}
	return cases
}

// rangeRunDataCases are data.txt applied to the data files Canon wrote under gen's project dir.
func rangeRunDataCases(t *testing.T, dir string) []rangeRunCase {
	t.Helper()
	var cases []rangeRunCase
	for _, f := range rangeRunRows(t, rangeRunDataFile) {
		c := rangeRunCase{name: f[0], kind: f[1], text: f[3], want: f[4]}
		if c.kind != rangeRunKindCaps { // a @text result is the whole text; the others edit the file Canon wrote
			raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rangeRunEmitted), f[1]+e2eJSONSuffix))
			if err != nil {
				t.Fatal(err)
			}
			from, to := strings.ReplaceAll(f[2], rangeRunNewline, "\n"), strings.ReplaceAll(f[3], rangeRunNewline, "\n")
			c.text = rangeRunEdit(t, f[0], string(raw), from, to)
		}
		cases = append(cases, c)
	}
	return cases
}

// rangeRunReader is one reader's verdicts, in the order of the cases it reads.
type rangeRunReader []string

// TestRangeRoundTrip is CODEGEN.md §5.15 and FINGERPRINT.md §5 end to end: on the same JSON, `load` and every reader of a Probe that holds a range or length refinement at every position agree, in Go, C++ and TypeScript. lib's types-mode decoders read the text (Go DecodeProbe, C++ Parse and Decode, TypeScript parseProbe and decodeProbe) and app's data loaders read the data file Canon writes of the value `load` gave; where `load` refuses the text, they read it wrapped with the true $schema, old data under a matching fingerprint (refinements are outside the fingerprint). Every reader says `ok` or the same `error <path>: <message>` as the case says and as every other reader, save the accepted integer-width text (rangeRunWidthForms) and the label of a data file. Keyed lists and tables of app are read as data files only.
func TestRangeRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs generated code")
	}
	dir := t.TempDir()
	gen := rangeRunBuild(t, dir)
	schema := rangeRunSchema(t, dir)
	cases := append(rangeRunProbeCases(t), rangeRunDataCases(t, dir)...)
	files := rangeRunFiles(t, dir, schema, cases)
	oracle := rangeRunOracle{widths: rangeRunWidthForms(t)}
	readers := map[string]rangeRunReader{}
	t.Run("go", func(t *testing.T) { maps.Copy(readers, rangeRunGo(t, gen, files)) })
	t.Run("cpp", func(t *testing.T) { maps.Copy(readers, rangeRunCpp(t, gen, files)) })
	t.Run("ts", func(t *testing.T) { maps.Copy(readers, rangeRunTS(t, gen, files)) })
	rangeRunCompare(t, oracle, cases, readers)
}

// rangeRunBuild builds the project of rangeRunProject in dir with no finding and returns its gen directory.
func rangeRunBuild(t *testing.T, dir string) string {
	t.Helper()
	writeProject(t, dir, rangeRunProject)
	if err := os.WriteFile(filepath.Join(dir, e2eProjFile), []byte(e2eFeatures), e2eFileMode); err != nil {
		t.Fatal(err)
	}
	res := rangeRunBuilt(t, dir, nil)
	if res.Summary.Errors > 0 {
		t.Fatalf("build findings:\n%s", renderFindings(t, res.Findings))
	}
	return filepath.Join(dir, e2eGenRoot)
}

// rangeRunBuilt builds the packages of dir (all, for none named); a Go error is a broken build, never `load` refusing a file.
func rangeRunBuilt(t *testing.T, dir string, packages []string) *build.BuildResult {
	t.Helper()
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{Packages: packages})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// rangeRunSchema is the $schema of app's data file of a Probe.
func rangeRunSchema(t *testing.T, dir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rangeRunSchemaFile)))
	if err != nil {
		t.Fatal(err)
	}
	var head struct {
		Schema string `json:"$schema"`
	}
	if err := json.Unmarshal(raw, &head); err != nil || head.Schema == "" {
		t.Fatalf("no $schema in %s: %v", rangeRunSchemaFile, err)
	}
	return head.Schema
}

// rangeRunFiles writes every case under dir, `<kind>:<path>` as the probes take them, and returns them in order.
func rangeRunFiles(t *testing.T, dir, schema string, cases []rangeRunCase) []string {
	t.Helper()
	cdir := filepath.Join(dir, rangeRunCasesDir)
	if err := os.MkdirAll(cdir, e2eDirMode); err != nil {
		t.Fatal(err)
	}
	var args []string
	for _, c := range cases {
		path := filepath.Join(cdir, c.name+e2eJSONSuffix)
		if err := os.WriteFile(path, []byte(c.text+"\n"), e2eFileMode); err != nil {
			t.Fatal(err)
		}
		if c.kind == rangeRunKindProbe {
			rangeRunTwin(t, dir, schema, c, path)
		}
		args = append(args, c.kind+rangeRunKindSep+path)
	}
	return args
}

// rangeRunTwin writes the .data twin of a probe case at path: the data file Canon writes of the value `load` gives (WIRE.md §8.1), or, where `load` refuses the text, the text wrapped as one under the true $schema.
func rangeRunTwin(t *testing.T, dir, schema string, c rangeRunCase, path string) {
	t.Helper()
	data, loads := rangeRunLoads(t, dir, c)
	if loads != c.loads {
		t.Errorf("%s: load accepts it: %v, want %v", c.name, loads, c.loads)
	}
	if !loads {
		data = []byte(`{"$schema": "` + schema + `", "value": ` + c.text + "}\n")
	}
	if err := os.WriteFile(path+rangeRunDataExt, data, e2eFileMode); err != nil {
		t.Fatal(err)
	}
}

// rangeRunLoads builds a package loading c's text as lib's Probe (WIRE.md §5) and returns the data file its json emit writes, and whether the build accepts the text.
func rangeRunLoads(t *testing.T, dir string, c rangeRunCase) ([]byte, bool) {
	t.Helper()
	pkg := filepath.Join(dir, rangeRunLoaderPkg)
	if err := os.MkdirAll(pkg, e2eDirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, rangeRunLoaderPkg+canonExt), []byte(rangeRunLoader), e2eFileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, rangeRunLoaderIn), []byte(c.text), e2eFileMode); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, filepath.FromSlash(rangeRunLoaderOut))
	if err := os.RemoveAll(filepath.Dir(out)); err != nil {
		t.Fatal(err)
	}
	if rangeRunBuilt(t, dir, []string{rangeRunLoaderPkg}).Summary.Errors > 0 {
		return nil, false
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data, true
}

// rangeRunCopy copies the probe source name to path.
func rangeRunCopy(t *testing.T, name, path string) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(rangeRunDir, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), e2eDirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, src, e2eFileMode); err != nil {
		t.Fatal(err)
	}
}

// rangeRunGo runs the Go probe and returns each reader's verdicts.
func rangeRunGo(t *testing.T, gen string, files []string) map[string]rangeRunReader {
	t.Helper()
	writeGoMod(t, gen, e2eGoModule)
	rangeRunCopy(t, rangeRunGoMain, filepath.Join(gen, rangeRunGoDir, rangeRunGoFile))
	cmd := exec.Command(goRunBinary, append([]string{rangeRunGoRun, "./" + rangeRunGoDir}, files...)...)
	return rangeRunReaders(t, refRunOutput(t, gen, "", cmd), rangeRunGoReaders, []string{""})
}

// rangeRunCpp compiles and runs the C++ probe with each compiler found, against lib's and app's .gen.cpp; without one it skips, or fails where the C++ toolchain is required.
func rangeRunCpp(t *testing.T, gen string, files []string) map[string]rangeRunReader {
	t.Helper()
	compilers, include := cxx.Toolchain(t)
	rangeRunCopy(t, rangeRunCppSrc, filepath.Join(gen, rangeRunCppMain))
	var out, tags []string
	for _, cc := range compilers {
		tag := filepath.Base(cc)
		bin := filepath.Join(gen, rangeRunCppBin+rangeRunTagSep+tag)
		args := append(slices.Clone(cxx.Flags), cppRunFlagI, include, cppRunFlagI, gen, cppRunFlagO, bin, rangeRunCppMain,
			filepath.Join(rangeRunLibDir, rangeRunLibSrc), filepath.Join(rangeRunAppDir, rangeRunAppSrc))
		runIn(t, gen, exec.Command(cc, args...))
		out = append(out, refRunOutput(t, gen, tag, exec.Command(bin, files...))...)
		tags = append(tags, tag)
	}
	return rangeRunReaders(t, out, rangeRunCppReaders, tags)
}

// rangeRunTS compiles lib.ts and app.ts with each compiler found and runs the TypeScript probe on them with node; without one it skips, or fails where TypeScript is required.
func rangeRunTS(t *testing.T, gen string, files []string) map[string]rangeRunReader {
	t.Helper()
	tc := tsc.Find(t)
	var out, tags []string
	for _, c := range tc.Compilers {
		tags = append(tags, c.Name)
		js := filepath.Join(gen, rangeRunTSOut+rangeRunTagSep+c.Name)
		args := append([]string{c.Script, rangeRunOutDirFlag, js}, tsc.Args(tc.Typings, rangeRunLibTS, rangeRunAppTS)...)
		runIn(t, gen, exec.Command(tc.Node, args...))
		rangeRunCopy(t, rangeRunTSSrc, filepath.Join(js, rangeRunTSMain))
		if err := os.WriteFile(filepath.Join(js, rangeRunTSPkg), []byte(rangeRunTSModule), e2eFileMode); err != nil {
			t.Fatal(err)
		}
		out = append(out, refRunOutput(t, gen, c.Name, exec.Command(tc.Node, append([]string{filepath.Join(js, rangeRunTSMain)}, files...)...))...)
	}
	return rangeRunReaders(t, out, rangeRunTSReaders, tags)
}

// rangeRunReaders groups probe lines, `<reader> <verdict>`, by reader, in the order printed, and fails the test for every reader of names, built with each of tags, that printed nothing (an empty tag is none).
func rangeRunReaders(t *testing.T, lines, names, tags []string) map[string]rangeRunReader {
	t.Helper()
	readers := map[string]rangeRunReader{}
	for _, line := range lines {
		reader, verdict, _ := strings.Cut(line, " ")
		readers[reader] = append(readers[reader], verdict)
	}
	for _, name := range names {
		for _, tag := range tags {
			full := name
			if tag != "" {
				full += rangeRunReaderSep + tag
			}
			if len(readers[full]) == 0 {
				t.Errorf("reader %s printed no verdict", full)
			}
		}
	}
	return readers
}

// rangeRunReads reports whether the reader named reader (`go-data`, `cpp-parse/g++`) reads case c: the readers of a @text result read a caps case alone, the types-mode and data readers a probe case, the data loaders an items or tags file.
func rangeRunReads(reader string, c rangeRunCase) bool {
	file := strings.Contains(reader, rangeRunFileTag)
	switch c.kind {
	case rangeRunKindCaps:
		return file
	case rangeRunKindProbe:
		return !file
	}
	return strings.Contains(reader, rangeRunDataTag)
}

// rangeRunCompared reports whether the verdict of a reader of c is judged: the parsed-value readers leave a text-only case out, and the TypeScript one alone a `tsdecode` case (ADR-0026 §15.10).
func rangeRunCompared(reader string, c rangeRunCase) bool {
	decode := strings.Contains(reader, rangeRunDecodeTag)
	tsDecode := decode && strings.HasPrefix(reader, rangeRunTSTag)
	return !(c.textOnly && decode) && !(c.tsDecodeOnly && tsDecode)
}

// rangeRunOracle is what the readers' verdicts are compared with; widths are the forms of rangeRunWidthForms.
type rangeRunOracle struct{ widths []*regexp.Regexp }

// rangeRunLabel is the label a types-mode decoder puts before its failures, which CODEGEN.md §5.13 fixes for every target: the type it reads, or the file of a @text result. A data loader names its file instead, a path in Go and C++ and `<name>.json` in TypeScript, so a data verdict carries none.
func rangeRunLabel(c rangeRunCase) string {
	if c.kind == rangeRunKindCaps {
		return rangeRunLabelCaps
	}
	return rangeRunLabelProbe
}

// rangeRunData reports whether reader is a data loader.
func rangeRunData(reader string) bool { return strings.Contains(reader, rangeRunDataTag) }

// verdict is a reader's raw verdict as `ok` or `error [<label>: ]<path after the root>: <message>`; the root of the path is `$` in a types-mode decoder and the `value` envelope in a probe's data file.
func (o rangeRunOracle) verdict(reader string, c rangeRunCase, raw string) string {
	if raw == rangeRunOK {
		return raw
	}
	label, rest, _ := strings.Cut(strings.TrimPrefix(raw, rangeRunErrorPre), rangeRunMsgSep)
	root := rangeRunTypesRoot
	if rangeRunData(reader) {
		label, root = "", ""
		if c.kind == rangeRunKindProbe {
			root = rangeRunEnvelope
		}
	}
	return o.format(reader, label, rest, root)
}

// want is the verdict case c wants of reader: its want, or its dataWant of a data loader, in the form reader's verdict is compared in.
func (o rangeRunOracle) want(c rangeRunCase, reader string) string {
	want, label := c.want, rangeRunLabel(c)
	if rangeRunData(reader) {
		label = ""
		if c.dataWant != "" {
			want = c.dataWant
		}
	}
	if want == rangeRunOK {
		return want
	}
	return o.format(reader, label, want, rangeRunTypesRoot)
}

// format is `error [<label>: ]<path>: <message>` for the `<path>: <message>` of text, root and its separator cut from the path. The message of a TypeScript reader that has the form of an integer width (rangeRunWidthForms) is rangeRunWidth; every other message, and every Go and C++ message, is as written.
func (o rangeRunOracle) format(reader, label, text, root string) string {
	path, msg, _ := strings.Cut(text, rangeRunMsgSep)
	path = strings.TrimPrefix(strings.TrimPrefix(path, root), rangeRunPathSep)
	if strings.HasPrefix(reader, rangeRunTSTag) && slices.ContainsFunc(o.widths, func(form *regexp.Regexp) bool { return form.MatchString(msg) }) {
		msg = rangeRunWidth
	}
	if label != "" {
		label += rangeRunMsgSep
	}
	return rangeRunErrorPre + label + path + rangeRunMsgSep + rangeRunFirstAt.Replace(msg)
}

// rangeRunCompare checks every reader's i-th verdict against case i that it reads: each says what the case wants of it, so all agree.
func rangeRunCompare(t *testing.T, o rangeRunOracle, cases []rangeRunCase, readers map[string]rangeRunReader) {
	t.Helper()
	if len(readers) == 0 {
		t.Fatal("no verdict")
	}
	names := slices.Sorted(maps.Keys(readers))
	next := map[string]int{}
	for _, c := range cases {
		got := map[string]string{}
		for _, reader := range names {
			if !rangeRunReads(reader, c) {
				continue
			}
			if next[reader] >= len(readers[reader]) {
				t.Fatalf("%s gave %d verdicts, fewer than its cases", reader, len(readers[reader]))
			}
			if rangeRunCompared(reader, c) {
				got[reader] = o.verdict(reader, c, readers[reader][next[reader]])
			}
			next[reader]++
		}
		rangeRunJudge(t, o, c, got)
	}
	for _, reader := range names {
		if next[reader] != len(readers[reader]) {
			t.Errorf("%s gave %d verdicts for %d cases", reader, len(readers[reader]), next[reader])
		}
	}
}

// rangeRunJudge reports a case on which a reader says other than its want.
func rangeRunJudge(t *testing.T, o rangeRunOracle, c rangeRunCase, got map[string]string) {
	t.Helper()
	differ := false
	var lines []string
	for _, reader := range slices.Sorted(maps.Keys(got)) {
		want := o.want(c, reader)
		differ = differ || got[reader] != want
		lines = append(lines, "\n\t"+reader+": "+got[reader]+" (want "+want+")")
	}
	if differ {
		t.Errorf("%s (%s):%s", c.name, c.kind, strings.Join(lines, ""))
	}
}
