package progen_test

import (
	"os"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

const shapesDir = "features/zzm/"

// shapesPackage is a data-mode package of several data values, as examples/features/shared is.
const shapesPackage = `/// Several data values.
package features.zzm

/// A row.
record RowOne {
  /// Its size.
  n: Int
}

/// A row.
record RowTwo {
  /// Its size.
  n: Int
}

/// A row.
record RowThree {
  /// Its size.
  n: Int
}

/// First.
let one: table RowOne = {
  alpha { n: 1 }
}

/// Second.
let two: table RowTwo = {
  beta { n: 2 }
}

/// Third.
let three: table RowThree = {
  gamma { n: 3 }
}

emit go { out: "out/go/zzm/", mode: data }
emit json { out: "out/data/" }
`

// shapesProject is the examples' project holding shapesPackage under the features root.
func shapesProject(t *testing.T, src string) (*progen.Project, target) {
	t.Helper()
	pc, err := os.ReadFile(examplesDir + "/" + projectFile)
	if err != nil {
		t.Fatal(err)
	}
	p := progen.NewProject()
	p.Set(projectFile, []byte(strings.Replace(string(pc), "studio: studio\n", "", 1)))
	p.Set(shapesDir+"zzm.canon", []byte(src))
	tg := metaTarget(shapesDir+"zzm.canon", src)
	tg.pkg = "features.zzm"
	tg.all = &[]target{tg}
	return p, tg
}

// indexOf is the index of the emit operator for code.
func indexOf(t *testing.T, code diag.Code) int {
	t.Helper()
	for i, o := range emitOperators() {
		if o.code == code {
			return i
		}
	}
	t.Fatalf("no operator %s", code)
	return 0
}

// WIRE.md §8.1 (E8153)
func TestDropJSONEmitSeveralValues(t *testing.T) {
	p, tg := shapesProject(t, shapesPackage)
	o := emitOperators()[indexOf(t, diag.E8153.Def().Code)]
	r := run{pkgs: []string{tg.pkg}}
	base := progen.Run(t.Context(), p, progen.RunOptions{Packages: r.pkgs, Roots: exampleRoots()})
	if len(base.Findings) > 0 {
		t.Fatalf("fixture is not clean: %s", describe(base.Findings))
	}
	sites := o.sites(tg)
	if len(sites) == 0 {
		t.Fatal("no site in a data-mode package of several values")
	}
	for _, s := range sites {
		s.Path = tg.path
		m, err := s.Mutate(p)
		if err != nil {
			t.Fatal(err)
		}
		if v := judge(o, r, m.At, m.Project, nil); v.Kind != "" {
			t.Errorf("%s: %s", v.Kind, v.Text)
		}
	}
}

// WIRE.md §2.2
func TestJSONWritersRelativeOut(t *testing.T) {
	a := metaTarget("m/sub/a.canon", "package m\nlet one: Int = 1\nemit json { out: \"out/data/\" }\n")
	b := metaTarget("n/b.canon", "package n\nlet two: Int = 2\nemit json { out: \"out/two.json\", values: [two] }\n")
	a.pkg, b.pkg = "m", "n"
	all := []target{a, b}
	for i := range all {
		all[i].all = &all
	}
	got := jsonWriters(all[1])
	if len(got) != 1 || got[0].file != "../m/sub/out/data/one.json" {
		t.Fatalf("writers %+v, want one writing ../m/sub/out/data/one.json", got)
	}
	rooted := metaTarget("m/a.canon", "package m\nlet one: Int = 1\nemit json { out: \"@web/d/\" }\n")
	rooted.pkg, rooted.all = "m", &all
	all[0] = rooted
	if got := jsonWriters(all[1]); len(got) != 1 || !strings.HasPrefix(got[0].file, "@web/d/") {
		t.Fatalf("writers %+v, want the rooted path kept", got)
	}
}
