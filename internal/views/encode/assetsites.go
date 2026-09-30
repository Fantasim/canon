package encode

import (
	"runtime"
	"sync"
	"weak"

	"github.com/fantasim/canonlang/internal/syntax"
)

// siteCache holds the asset types each file writes (VIEWMODEL.md 12.3 `asset.root`): data derived
// only from an immutable file, held weakly, never ranged over; it changes no output, only its time.
type siteCache struct {
	mu    sync.Mutex
	files map[weak.Pointer[syntax.File]][]*syntax.AssetType
}

// sites is the process's cache of asset types by file.
var sites siteCache

// assetTypesOf is the asset types f writes, in source order.
func assetTypesOf(f *syntax.File) []*syntax.AssetType {
	return sites.typesOf(f)
}

// typesOf is the asset types f writes, in source order, f walked on the first ask only.
func (c *siteCache) typesOf(f *syntax.File) []*syntax.AssetType {
	key := weak.Make(f)
	c.mu.Lock()
	found, ok := c.files[key]
	c.mu.Unlock()
	if ok {
		return found
	}
	found = scanAssetTypes(f)
	c.mu.Lock()
	defer c.mu.Unlock()
	if kept, ok := c.files[key]; ok {
		return kept
	}
	if c.files == nil {
		c.files = map[weak.Pointer[syntax.File]][]*syntax.AssetType{}
	}
	c.files[key] = found
	runtime.AddCleanup(f, c.forget, key)
	return found
}

// forget drops the entry of a file that no longer exists.
func (c *siteCache) forget(key weak.Pointer[syntax.File]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.files, key)
}

// scanAssetTypes walks f for its asset types.
func scanAssetTypes(f *syntax.File) []*syntax.AssetType {
	var out []*syntax.AssetType
	syntax.Inspect(f, func(n syntax.Node) bool {
		if at, ok := n.(*syntax.AssetType); ok {
			out = append(out, at)
		}
		return true
	})
	return out
}
