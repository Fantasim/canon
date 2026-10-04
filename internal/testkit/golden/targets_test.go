package golden

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// defaultTargets are the targets an example without its own row in exampleTargets is built for.
var defaultTargets = []canon.Target{canon.TargetGo, canon.TargetTS, canon.TargetJSON}

// exampleTargets overrides defaultTargets; an example with a row must have expected/MANIFEST (embedded: no Go emit, gen/go has no embedded mode).
var exampleTargets = map[string][]canon.Target{
	"pipeline":           {canon.TargetGo, canon.TargetCpp, canon.TargetJSON, canon.TargetView},
	"features.dependent": {canon.TargetGo, canon.TargetCpp, canon.TargetJSON},
	"features.copies":    {canon.TargetGo, canon.TargetCpp, canon.TargetTS, canon.TargetJSON},
	"features.lookup":    {canon.TargetGo, canon.TargetTS},
	"features.embedded":  {canon.TargetTS},
	"features.textemit":  {canon.TargetGo, canon.TargetCpp, canon.TargetTS, canon.TargetJSON, canon.TargetText},
	"resource.farm":      {canon.TargetGo, canon.TargetJSON, canon.TargetView},
	"resource.events":    {canon.TargetGo, canon.TargetJSON, canon.TargetView},
	"resource.vocab":     {canon.TargetCpp, canon.TargetView},
	"telemetry":          {canon.TargetGo, canon.TargetCpp, canon.TargetText},
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
	slices.Sort(manifests)
	return manifests
}
