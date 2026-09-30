package build

import (
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

const (
	benchAssetNames = 7000
	benchAssetDir   = "Icons"
	benchAssetRoot  = "@assets"
	benchAssetFrom  = "items"
	benchAssetExt   = ".png"
	benchAssetStem  = "icon_"
)

// BenchmarkAssetExists: warm checks against a 7,000-name listing (TYPES.md §13.4).
func BenchmarkAssetExists(b *testing.B) {
	files := ciDir{}
	for i := range benchAssetNames {
		files[benchAssetStem+strconv.Itoa(i)+benchAssetExt] = nil
	}
	tree := ciDir{"assets": ciDir{benchAssetDir: files}}
	p := &project.Project{Roots: []project.Root{{Name: "assets", Path: "assets"}}}
	layout, ok := project.NewLayout(p, "/", nil, diag.NewBag(nil, ""))
	if !ok {
		b.Fatal("layout")
	}
	a := &assets{fs: ciFS(tree), layout: layout, host: &evalHost{}, dirs: map[string]dirListing{}}
	hit := benchAssetDir + "/" + benchAssetStem + strconv.Itoa(benchAssetNames-1) + benchAssetExt
	miss := benchAssetDir + "/" + benchAssetStem + strconv.Itoa(benchAssetNames) + benchAssetExt
	if _, found := a.Exists(benchAssetRoot, benchAssetFrom, hit); !found {
		b.Fatal("hit not found")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		a.Exists(benchAssetRoot, benchAssetFrom, hit)
		a.Exists(benchAssetRoot, benchAssetFrom, miss)
	}
}
