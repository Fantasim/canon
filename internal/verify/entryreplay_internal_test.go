package verify

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

const (
	replayRoot = "@client/Icon"
	replayName = "sword.png"
)

// flipAssets is an Assets whose root can turn absent.
type flipAssets struct{ absent bool }

func (f *flipAssets) Exists(root, _, name string) (string, bool) { return root, name == replayName }

func (f *flipAssets) Absent(string, string) (string, bool) { return "client", f.absent }

// TYPES.md §13.4: an asset lookup is not replayed once its root turns absent or present.
func TestSameReadAssetRootPresence(t *testing.T) {
	spec := &types.AssetSpec{Root: replayRoot}
	for _, start := range []bool{false, true} {
		for _, name := range []string{replayName, "shield.png"} {
			flip := &flipAssets{absent: start}
			w := &walker{Verifier: &Verifier{assets: flip, src: &sources{assetDirs: map[*types.AssetSpec]string{}}}}
			rd := entryRead{asset: spec, name: name, look: w.findAsset(spec, name)}
			if !w.sameRead(rd) {
				t.Errorf("absent=%t %s: not the same asked again", start, name)
			}
			flip.absent = !start
			if w.sameRead(rd) {
				t.Errorf("absent=%t %s: replayed after the root turned %t", start, name, flip.absent)
			}
			flip.absent = start
			if !w.sameRead(rd) {
				t.Errorf("absent=%t %s: not the same once the root is back", start, name)
			}
		}
	}
}
