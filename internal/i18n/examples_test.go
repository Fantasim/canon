package i18n_test

import (
	"context"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	examplesDir = "../../examples"
	projectFile = "project.canon"
)

// exampleRoots are the roots examples/project.canon declares (internal/check/examples_test.go).
var exampleRoots = []string{"client", "features", "generated", "parity", "pipeline_go", "resource", "services", "source", "sovcommon", "web"}

// exampleProject is the project of examples/project.canon, as far as check and i18n read it.
func exampleProject() *project.Project {
	p := project.New("sovereign", project.Version{Major: 0, Minor: 1})
	p.Languages = []string{"en", "fr"}
	p.Studio = project.Package{Path: "studio"}
	for _, r := range exampleRoots {
		p.Roots = append(p.Roots, project.Root{Name: r, Path: r})
	}
	return p
}

// loadEveryExample parses every .canon file under examples/, in path order.
func loadEveryExample(t *testing.T) (*source.FileSet, []*syntax.File) {
	t.Helper()
	fset := &source.FileSet{}
	parse := diag.NewBag(fset, "")
	var files []*syntax.File
	err := filepath.WalkDir(examplesDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != canonExt || d.Name() == projectFile {
			return err
		}
		rel, err := filepath.Rel(examplesDir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		src, err := fset.Add(rel, "/"+rel, data)
		if err != nil {
			return err
		}
		files = append(files, syntax.Parse(src, syntax.FileSource, parse))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return fset, files
}

// draftedRe matches a pre-drafted missing-translation line's count and package (I18N.md §11).
var draftedRe = regexp.MustCompile(`(\d+) texts? of package (\S+) have? no fr translation`)

// draftedMissing is every package's pre-drafted W1701 count, read from every expected/findings.txt
// under examples/.
func draftedMissing(t *testing.T) map[string]int {
	t.Helper()
	out := map[string]int{}
	err := filepath.WalkDir(examplesDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "findings.txt" {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if m := draftedRe.FindSubmatch(data); m != nil {
			n, err := strconv.Atoi(string(m[1]))
			if err != nil {
				return err
			}
			out[string(m[2])] = n
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestExampleCatalogueCounts checks every example's fr-missing count (I18N.md §11).
func TestExampleCatalogueCounts(t *testing.T) {
	_, files := loadEveryExample(t)
	bags := check.Bags{}
	proj := exampleProject()
	prog := check.Check(context.Background(), proj, files, bags, eval.NewFolder(bags, eval.Options{}))
	if prog == nil {
		t.Fatal("check.Check returned nil")
	}
	res := i18n.Check(prog, proj, bags, emitsView(prog))

	checkFarmAndPipeline(t, res)
	drafted := draftedMissing(t)
	for _, name := range slices.Sorted(maps.Keys(drafted)) {
		if res[name] == nil {
			t.Errorf("%s: drafted in a findings.txt but not loaded", name)
			continue
		}
		fr := res[name].Languages["fr"]
		if got := fr.Missing; got != drafted[name] {
			t.Errorf("%s: computed %d missing fr texts (catalogue %d, translated %d), drafted findings.txt says %d",
				name, got, len(res[name].Catalogue.Entries), len(fr.Texts), drafted[name])
		}
	}
}

// checkFarmAndPipeline asserts the two normative counts of I18N.md §11 exactly.
func checkFarmAndPipeline(t *testing.T, res map[string]*i18n.Result) {
	t.Helper()
	farm := res["resource.farm"]
	if farm == nil {
		t.Fatal("no result for resource.farm")
	}
	if got := len(farm.Catalogue.Entries); got != 88 {
		t.Errorf("resource.farm: catalogue has %d keys, want 88 (I18N.md §11)", got)
	}
	if got := len(farm.Languages["fr"].Texts); got != 49 {
		t.Errorf("resource.farm: %d fr texts translated, want 49 (I18N.md §11)", got)
	}
	if got := farm.Languages["fr"].Missing; got != 39 {
		t.Errorf("resource.farm: %d fr texts missing, want 39 (I18N.md §11)", got)
	}

	pipeline := res["pipeline"]
	if pipeline == nil {
		t.Fatal("no result for pipeline")
	}
	want := []string{
		"Potion.check.fast_big_heal", "Potion.cooldown", "Potion.cooldown.help", "Potion.heal",
		"Potion.heal.help", "Potion.help", "Potion.id", "Potion.id.help", "Potion.name", "Potion.name.help",
		"Potion.stack", "Potion.stack.help", "potions", "potions.help",
	}
	var got []string
	for _, e := range pipeline.Catalogue.Entries {
		got = append(got, e.Key)
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("pipeline: keys = %v, want %v (I18N.md §11)", got, want)
	}
}
