package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const (
	viewTopic      = "views-i18n"
	viewFile       = "shop/out/shop.view.json"
	fieldSep       = "."
	kindTags       = "tags"
	kindPositional = "positional"
	kindEnumRow    = "enumRow"
	kindCards      = "cards"
)

// controlKinds are the four controls `canon guide views-i18n` states as a contract.
var controlKinds = []string{kindCards, kindEnumRow, kindPositional, kindTags}

// viewModel is the part of a view model the guide's control fences are compared with.
type viewModel struct {
	Views map[string]struct {
		Fields map[string]struct {
			Control any `json:"control"`
		} `json:"fields"`
	} `json:"views"`
}

// VIEWMODEL.md §12.5, §7.5, §7.6: each `json view/<type>.<field>` fence is the control built for it.
func TestGuideControlsJSON(t *testing.T) {
	dir := writeFiles(t, guideProjects(t)[viewTopic])
	buildGuideProject(t, dir)
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(viewFile)))
	if err != nil {
		t.Fatal(err)
	}
	var doc viewModel
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, f := range allFences(t) {
		spec, ok := strings.CutPrefix(f.arg, viewDir)
		if !ok || f.topic != viewTopic {
			continue
		}
		cut := strings.LastIndex(spec, fieldSep)
		if cut < 0 {
			t.Errorf("%s: %q is not <type>.<field>", f, spec)
			continue
		}
		var want map[string]any
		if err := json.Unmarshal([]byte(f.body), &want); err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		kind, _ := want["kind"].(string)
		kinds = append(kinds, kind)
		if got := doc.Views[spec[:cut]].Fields[spec[cut+1:]].Control; !reflect.DeepEqual(got, any(want)) {
			t.Errorf("%s: the compiler writes %v", f, got)
		}
	}
	slices.Sort(kinds)
	if !slices.Equal(kinds, controlKinds) {
		t.Errorf("controls stated %v, want %v", kinds, controlKinds)
	}
}
