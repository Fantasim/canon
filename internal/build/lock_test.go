package build_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

// teamboardFS is examples/teamboard and its imports in memory under /ex, taxonomy.canon edited
// by the replacements (old, new…), with the golden canon.lock.
func teamboardFS(t *testing.T, edits ...string) mapFS {
	t.Helper()
	fsys := mapFS{}
	for _, name := range []string{"project.canon", "teamboard/taxonomy.canon", "sovcommon/roles/roles.canon", "sovcommon/ui/ui.canon", "sovcommon/time/time.canon", "studio/studio.canon"} {
		data, err := os.ReadFile(filepath.Join(examplesDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(name, "teamboard/") {
			data = []byte(strings.NewReplacer(edits...).Replace(string(data)))
		}
		fsys["ex/"+name] = file(string(data))
	}
	fsys["ex/teamboard/canon.lock"] = file(teamboardGolden(t, "canon.lock"))
	return fsys
}

func codes(list []diag.Finding) []diag.Code {
	var out []diag.Code
	for _, f := range list {
		out = append(out, f.Code)
	}
	return out
}

// IMPLEMENTATION-PLAN §6 M1 item 5, LOCK.md §5, §9.3–§9.5.
func TestTeamboardLock(t *testing.T) {
	retire := []string{"  duplicate {", "  retired duplicate {", "wont_do, duplicate]", "wont_do]"}
	for _, c := range []struct {
		name   string
		edits  []string
		codes  []diag.Code
		status build.Status
		lines  []string
	}{
		{"unchanged", nil, nil, build.StatusUnchanged, nil},
		{"delete", []string{"  wont_do { tone: neutral, label: \"Won't do\", terminal: true, next: [open], requires: [reason] }\n", "", "wont_do, ", ""},
			[]diag.Code{diag.E6001.Def().Code}, 0, nil},
		{"rename", []string{"wont_do", "rejected"}, []diag.Code{diag.E6001.Def().Code}, 0, nil},
		{"retire", retire, nil, build.StatusStale, []string{"table  teamboard.statuses  duplicate  retired"}},
	} {
		p, err := build.Open(teamboardFS(t, c.edits...), "/ex", build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Build(context.Background(), build.BuildOptions{Packages: []string{"teamboard"}, Targets: goAndJSON, Check: true})
		if err != nil {
			t.Fatal(err)
		}
		if got := codes(res.List); !slices.Equal(got, c.codes) {
			t.Errorf("%s: findings %v, want %v", c.name, got, c.codes)
		}
		if c.codes != nil {
			if len(res.Outputs) != 0 || len(res.Locks) != 0 {
				t.Errorf("%s: an error build returned %d outputs, %d locks", c.name, len(res.Outputs), len(res.Locks))
			}
			continue
		}
		if len(res.Locks) != 1 || res.Locks[0].Status != c.status || !slices.Equal(res.Locks[0].Lines, c.lines) {
			t.Errorf("%s: locks %+v", c.name, res.Locks)
		}
	}
}

// LOCK.md §2.4, §4.5.
func TestBadLock(t *testing.T) {
	fsys := teamboardFS(t, "wont_do", "rejected")
	fsys["ex/teamboard/canon.lock"] = file("# canon.lock v9\n")
	p, err := build.Open(fsys, "/ex", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{Packages: []string{"teamboard"}, Targets: goAndJSON, Check: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := codes(res.List); !slices.Equal(got, []diag.Code{diag.E6005.Def().Code}) || res.Outputs != nil || res.Locks != nil {
		t.Errorf("findings %v, %d outputs", got, len(res.Outputs))
	}
}
