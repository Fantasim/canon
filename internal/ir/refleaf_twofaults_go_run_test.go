package ir_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	twoFaultsPath = "testdata/e2e/gorun/two_faults.txtar"
	twoFaultsList = "cases.txt"
	twoFaultsTab  = "\t"
	twoFaultsData = "data"
	twoFaultsType = "types"
)

// twoFaultsReader is a reader of another package's class: its name in the probes, the holder file its json emit wrote (the data readers'; the types readers take the same value), and how its files are laid out.
type twoFaultsReader struct {
	name, holder, kind string
}

// twoFaultsReaders: rd and ty read ow.H, ending in an enum; rd2 and ty2 read ow2.H, ending in the ids of a table of lt (CODEGEN.md §5.3, §5.8, §5.13).
var twoFaultsReaders = []twoFaultsReader{
	{"rd", "rd/out/h.json", twoFaultsData}, {"ty", "rd/out/h.json", twoFaultsType},
	{"rd2", "rd2/out/h.json", twoFaultsData}, {"ty2", "rd2/out/h.json", twoFaultsType},
}

// twoFaultsCase is a file of a reader: the holder value with up to two replacements, and the first error both loaders give for it ("" for none).
type twoFaultsCase struct {
	name, reader string
	edits        [2][2]string
	firstError   string
}

// twoFaultsCases: a ref's key is read at the chain's leaf, and a data reader checks it against the owner's outermost list once, after the object, as C++ does; a types reader checks no key (CODEGEN.md §5.8, §5.13, DECISIONS 456). A fault in how the leaf is read comes first, in field order; a type error of a later field comes before the check of an earlier ref; of two refs the map key, checked as it is read, comes before the field, checked after the object. The value is written compact: `"f":"z2"`.
var twoFaultsCases = []twoFaultsCase{
	{"good", "rd", [2][2]string{}, ""},
	{"ref_then_type", "rd", [2][2]string{{`"f":"z2"`, `"f":"first"`}, {`"n":1`, `"n":"x"`}}, "value.n: expected an integer"},
	{"two_refs", "rd", [2][2]string{{`"f":"z2"`, `"f":"first"`}, {`"z2":1`, `"first":1`}}, "value.fm.first: no entry first"},
	{"unknown_then_type", "rd", [2][2]string{{`"f":"z2"`, `"f":"zz"`}, {`"n":1`, `"n":"x"`}}, "value.f: unknown value zz"},
	{"name_then_type", "rd", [2][2]string{{`"f":"z2"`, `"f":"second"`}, {`"n":1`, `"n":"x"`}}, "value.f: unknown value second"},
	{"key_then_type", "rd", [2][2]string{{`"z2":1`, `"first":1`}, {`"n":1`, `"n":"x"`}}, "value.fm.first: no entry first"},
	{"ty_good", "ty", [2][2]string{}, ""},
	{"ty_ref_then_type", "ty", [2][2]string{{`"f":"z2"`, `"f":"first"`}, {`"n":1`, `"n":"x"`}}, "W: $.h.n: expected an integer"},
	{"ty_unknown_then_type", "ty", [2][2]string{{`"f":"z2"`, `"f":"zz"`}, {`"n":1`, `"n":"x"`}}, "W: $.h.f: unknown value zz"},
	{"ty_name_then_type", "ty", [2][2]string{{`"f":"z2"`, `"f":"second"`}, {`"n":1`, `"n":"x"`}}, "W: $.h.f: unknown value second"},
	{"ty_key_then_type", "ty", [2][2]string{{`"z2":1`, `"first":1`}, {`"n":1`, `"n":"x"`}}, "W: $.h.fm.first: no entry first"},
	{"t_good", "rd2", [2][2]string{}, ""},
	{"t_ref_then_type", "rd2", [2][2]string{{`"f":"sword"`, `"f":"shield"`}, {`"n":1`, `"n":"x"`}}, "value.n: expected an integer"},
	{"t_two_refs", "rd2", [2][2]string{{`"f":"sword"`, `"f":"shield"`}, {`"sword":1`, `"shield":1`}}, "value.fm.shield: no entry shield"},
	{"t_unknown_then_type", "rd2", [2][2]string{{`"f":"sword"`, `"f":"zz"`}, {`"n":1`, `"n":"x"`}}, "value.f: no entry zz"},
	{"t_key_then_type", "rd2", [2][2]string{{`"sword":1`, `"shield":1`}, {`"n":1`, `"n":"x"`}}, "value.fm.shield: no entry shield"},
	{"ty2_good", "ty2", [2][2]string{}, ""},
	{"ty2_ref_then_type", "ty2", [2][2]string{{`"f":"sword"`, `"f":"shield"`}, {`"n":1`, `"n":"x"`}}, "W: $.h.n: expected an integer"},
	{"ty2_unknown_then_type", "ty2", [2][2]string{{`"f":"sword"`, `"f":"zz"`}, {`"n":1`, `"n":"x"`}}, "W: $.h.f: no entry zz"},
}

// twoFaultsValue is the compact holder value and the schema of the data file a reader's json emit wrote.
func twoFaultsValue(t *testing.T, dir string, r twoFaultsReader) (value, schema string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(r.holder)))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Schema string          `json:"$schema"`
		Value  json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, doc.Value); err != nil {
		t.Fatal(err)
	}
	return compact.String(), doc.Schema
}

// twoFaultsWrite writes the cases and their list into dir and returns the probe arguments, `reader:file`.
func twoFaultsWrite(t *testing.T, dir string) []string {
	t.Helper()
	cases := filepath.Join(dir, cppRunCases)
	if err := os.MkdirAll(cases, e2eDirMode); err != nil {
		t.Fatal(err)
	}
	var args, list []string
	for _, c := range twoFaultsCases {
		var r twoFaultsReader
		for _, x := range twoFaultsReaders {
			if x.name == c.reader {
				r = x
			}
		}
		value, schema := twoFaultsValue(t, dir, r)
		for _, e := range c.edits {
			if e[0] == "" {
				continue
			}
			if !strings.Contains(value, e[0]) {
				t.Fatalf("%s: %q is not in %s", c.name, e[0], value)
			}
			value = strings.Replace(value, e[0], e[1], 1)
		}
		body := `{"h":` + value + `}`
		if r.kind == twoFaultsData {
			body = `{"$schema":"` + schema + `","value":` + value + `}`
		}
		if err := os.WriteFile(filepath.Join(cases, c.name), []byte(body), e2eFileMode); err != nil {
			t.Fatal(err)
		}
		args = append(args, c.reader+":"+c.name)
		list = append(list, c.name+twoFaultsTab+c.reader+twoFaultsTab+c.firstError)
	}
	if err := os.WriteFile(filepath.Join(dir, twoFaultsList), []byte(strings.Join(list, "\n")+"\n"), e2eFileMode); err != nil {
		t.Fatal(err)
	}
	return args
}

// twoFaultsWant is what the C++ probe prints for the cases: `<file>: ok`, or the first error after the path for a data reader.
func twoFaultsWant() string {
	var want strings.Builder
	for _, c := range twoFaultsCases {
		switch {
		case c.firstError == "":
			want.WriteString(c.name + ": ok\n")
		case strings.HasPrefix(c.reader, "rd"):
			want.WriteString(c.name + ": error " + c.name + ": " + c.firstError + "\n")
		default:
			want.WriteString(c.name + ": error " + c.firstError + "\n")
		}
	}
	return want.String()
}

// TestTwoFaultsRun is DECISIONS 456 and CODEGEN.md §5.8, §5.13 (first failure in order) built and run on both targets: the Go loader and the C++ loader of a data reader and of a types reader of another package's class, whose refs go through a list keyed by a ref ending in an enum or in a table's ids, give the same first error for a file with two faults.
func TestTwoFaultsRun(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("compiles and runs generated code")
	}
	dir := cppRunBuild(t, twoFaultsPath)
	args := twoFaultsWrite(t, dir)
	want := twoFaultsWant()
	for _, got := range cppRunOutputs(t, dir, args) {
		if got != want {
			t.Errorf("c++ got:\n%s\nwant:\n%s", got, want)
		}
	}
	gen := filepath.Join(dir, e2eGenRoot)
	writeGoMod(t, gen, e2eGoModule)
	runIn(t, gen, exec.Command(goRunBinary, goRunTest, goRunNoCache, goRunAll))
}
