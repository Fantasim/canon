package golden

import (
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// defaultTargets are the targets an example without its own row in exampleTargets is built for.
var defaultTargets = []canon.Target{canon.TargetGo, canon.TargetJSON}

// exampleTargets overrides defaultTargets for the examples whose MANIFEST freezes more: the cpp
// data emits of pipeline and features.dependent, and the emit view of pipeline, resource.farm
// and resource.events (M3 acceptance 2). An example with a row must have expected/MANIFEST.
var exampleTargets = map[string][]canon.Target{
	"pipeline":           {canon.TargetGo, canon.TargetCpp, canon.TargetJSON, canon.TargetView},
	"features.dependent": {canon.TargetGo, canon.TargetCpp, canon.TargetJSON},
	"resource.farm":      {canon.TargetGo, canon.TargetJSON, canon.TargetView},
	"resource.events":    {canon.TargetGo, canon.TargetJSON, canon.TargetView},
}

// targetsFor is exampleTargets[name], or defaultTargets without a row.
func targetsFor(name string) []canon.Target {
	if t, ok := exampleTargets[name]; ok {
		return t
	}
	return defaultTargets
}

// withTargetManifests adds to manifests the expected/MANIFEST of every example exampleTargets
// names that has none yet: -update creates it; without -update its absence fails the test, so a
// row whose MANIFEST is deleted cannot silently stop freezing its outputs.
func withTargetManifests(t *testing.T, root string, manifests []string) []string {
	t.Helper()
	for _, name := range slices.Sorted(maps.Keys(exampleTargets)) {
		m := filepath.Join(root, filepath.FromSlash(strings.ReplaceAll(name, ".", "/")), expectedDir, manifestFile)
		if slices.Contains(manifests, m) {
			continue
		}
		if !*update {
			t.Errorf("%s: exampleTargets builds it but it has no %s (run -update)", name, manifestFile)
			continue
		}
		manifests = append(manifests, m)
	}
	sort.Strings(manifests)
	return manifests
}
