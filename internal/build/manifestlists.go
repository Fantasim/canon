package build

import (
	"path"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// listed is a directory an asset check listed: its display and its listing.
type listed struct {
	display string
	names   dirListing
}

// hostRef is a way an asset check of host h reached a listed directory.
type hostRef struct {
	h   *evalHost
	ref listRef
}

// assetAt is an asset root as written, and the directory of the file it is written in.
type assetAt struct {
	root, from string
}

// listings is every directory the hosts' asset checks listed, once, by its first asset site's display (WIRE.md §2.3).
func listings(hosts []*evalHost) []listed {
	refs := map[string][]hostRef{}
	for _, h := range hosts {
		if h.assets == nil {
			continue
		}
		//canon:unordered gathered by name; each name's display is chosen by site
		for abs, rs := range h.assets.shown {
			for _, ref := range rs {
				refs[abs] = append(refs[abs], hostRef{h: h, ref: ref})
			}
		}
	}
	sites := map[*check.Program]map[assetAt]siteKey{}
	out := make([]listed, 0, len(refs))
	//canon:unordered writeSums sorts the lines
	for abs, rs := range refs {
		out = append(out, listed{display: firstDisplay(rs, sites), names: rs[0].h.assets.dirs[abs]})
	}
	return out
}

// firstDisplay is the display the asset spec declared first in package then source order gave, the least on a tie.
func firstDisplay(rs []hostRef, sites map[*check.Program]map[assetAt]siteKey) string {
	best, bestSite := rs[0].ref.display, siteKey{}
	if !slices.ContainsFunc(rs, func(r hostRef) bool { return r.ref.display != best }) {
		return best
	}
	for i, r := range rs {
		prog := r.h.prog
		if sites[prog] == nil {
			sites[prog] = assetSites(prog)
		}
		site := sites[prog][assetAt{root: r.ref.root, from: r.ref.from}]
		if i == 0 || site.before(bestSite) || site == bestSite && r.ref.display < best {
			best, bestSite = r.ref.display, site
		}
	}
	return best
}

// assetSites is the first site, in package then source order, of each asset root of prog as written.
func assetSites(prog *check.Program) map[assetAt]siteKey {
	out := map[assetAt]siteKey{}
	for _, cp := range prog.Packages {
		for _, f := range cp.Files {
			syntax.Inspect(f, func(n syntax.Node) bool {
				noteAssetSite(prog, cp.Path, f, n, out)
				return true
			})
		}
	}
	return out
}

// noteAssetSite keeps n's site in out when n is an asset type of prog written before the others of its root.
func noteAssetSite(prog *check.Program, pkg string, f *syntax.File, n syntax.Node, out map[assetAt]siteKey) {
	at, ok := n.(*syntax.AssetType)
	if !ok {
		return
	}
	t, ok := prog.Info.TypeExprs[at].(*types.Refined)
	if !ok || t.Asset == nil {
		return
	}
	key := assetAt{root: t.Asset.Root, from: path.Dir(f.Src.Path)}
	site := siteKey{pkg: pkg, file: f.Src.Path, at: int(f.Span(at).Start)}
	if had, seen := out[key]; !seen || site.before(had) {
		out[key] = site
	}
}
