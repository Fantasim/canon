package build

import (
	"context"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
)

// The junction cases' tree: a project /p whose asset root is /p/assets, a directory /p/real and
// a directory /outside, each holding junctionPNG.
const (
	junctionProj    = "/p"
	junctionAssets  = "/p/assets"
	junctionReal    = "/p/real"
	junctionOutside = "/outside"
	junctionGhost   = "/p/ghost"
	junctionPNG     = "sword.png"
	junctionName    = "j"
	placeholderName = "ph.png"
	junctionRule    = "WIRE.md §6.5, TYPES.md §13.4, DECISIONS 419: "
	volumePrefix    = "//?/"
	volumeTarget    = volumePrefix + "Volume{G}/"
)

// junctionFS is a read-only tree whose irregular entry with data is a junction to it, followed as the OS does; one without, a placeholder.
type junctionFS struct{ m fstest.MapFS }

// newJunctionFS is files (name to content) and junctions (name to target), names absolute; a
// junction with an empty target is a placeholder.
func newJunctionFS(files, junctions map[string]string) junctionFS {
	m := fstest.MapFS{}
	for name, data := range files { //canon:unordered each file added alone
		m[roRel(name)] = srcFile(data)
	}
	for name, target := range junctions { //canon:unordered each junction added alone
		m[roRel(name)] = &fstest.MapFile{Data: []byte(target), Mode: fs.ModeIrregular}
	}
	return junctionFS{m}
}

// junctionOf is the target of the junction at name, false when name is none.
func (j junctionFS) junctionOf(name string) (string, bool) {
	f := j.m[roRel(name)]
	if f == nil || f.Mode&fs.ModeIrregular == 0 || len(f.Data) == 0 {
		return "", false
	}
	return string(f.Data), true
}

// EvalSymlinks is name with every junction on it followed; a missing result is fs.ErrNotExist.
func (j junctionFS) EvalSymlinks(name string) (string, error) {
	done, todo := "/", strings.Split(roRel(name), "/")
	for hops := 0; len(todo) > 0; {
		next := path.Join(done, todo[0])
		todo = todo[1:]
		target, ok := j.junctionOf(next)
		if !ok {
			done = next
			continue
		}
		if hops++; hops > project.MaxSymlinkHops {
			return "", project.ErrSymlinkLoop
		}
		if strings.HasPrefix(target, volumePrefix) {
			return "", project.ErrDeviceTarget
		}
		done, todo = "/", append(strings.Split(roRel(target), "/"), todo...)
	}
	if _, err := j.m.Stat(roRel(done)); err != nil {
		return "", err
	}
	return done, nil
}

// Readlink is the target of the junction name, as os.Readlink reads one.
func (j junctionFS) Readlink(name string) (string, error) {
	if target, ok := j.junctionOf(name); ok {
		return target, nil
	}
	return "", fs.ErrInvalid
}

func (j junctionFS) ReadFile(name string) ([]byte, error) {
	real, err := j.EvalSymlinks(name)
	if err != nil {
		return nil, err
	}
	return j.m.ReadFile(roRel(real))
}

// Stat follows junctions; a placeholder fails, as a cloud placeholder's may, so a reader that stats one shows.
func (j junctionFS) Stat(name string) (fs.FileInfo, error) {
	real, err := j.EvalSymlinks(name)
	if err != nil {
		return nil, err
	}
	info, err := j.m.Stat(roRel(real))
	if err == nil && info.Mode()&fs.ModeIrregular != 0 {
		return nil, fs.ErrPermission
	}
	return info, err
}

func (j junctionFS) ReadDir(name string) ([]fs.DirEntry, error) {
	real, err := j.EvalSymlinks(name)
	if err != nil {
		return nil, err
	}
	return j.m.ReadDir(roRel(real))
}

// junctionProject is a project with the asset root assets, whose package items checks one icon.
const junctionProject = "project demo {\n  canon: \"0.1\"\n  roots {\n    assets: \"assets\"\n  }\n}\n"

// junctionSource is package items, whose one item's icon is icon under root.
func junctionSource(root, icon string) string {
	return "/// Items.\npackage items\n\n/// Item.\nrecord Item {\n  /// Icon.\n  icon: asset(\"" + root + "\", ext: [png])\n}\n\n" +
		"/// Items.\nlet items: stable table Item = {\n  sword { icon: \"" + icon + "\" }\n}\n"
}

// junctionTree is the junction cases' files, the item's icon at icon under root, and the junctions given.
func junctionTree(root, icon string, junctions map[string]string) junctionFS {
	return newJunctionFS(map[string]string{
		junctionProj + "/project.canon":           junctionProject,
		junctionProj + "/items/items.canon":       junctionSource(root, icon),
		junctionReal + "/" + junctionPNG:          "x",
		junctionOutside + "/" + junctionPNG:       "x",
		junctionOutside + "/sub/" + junctionPNG:   "x",
		junctionProj + "/real/sub/" + junctionPNG: "x",
	}, junctions)
}

// assetCodes are the codes a check of fsys's project reports.
func assetCodes(t *testing.T, fsys project.FS) []diag.Code {
	t.Helper()
	p, err := Open(fsys, junctionProj, Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var out []diag.Code
	for _, f := range res.Findings.List {
		out = append(out, f.Code)
	}
	return out
}

// WIRE.md §6.5, TYPES.md §13.4, DECISIONS 419: a junction inside the bounds is followed, outside E3706, dangling E3701; a placeholder is no link.
func TestAssetJunction(t *testing.T) {
	inJunction, nested, rootJ := junctionName+"/"+junctionPNG, junctionName+"/sub/"+junctionPNG, linkRootName+"/"+junctionName
	j := junctionAssets + "/" + junctionName
	for _, c := range []struct {
		name, root, icon string
		junctions        map[string]string
		want             []diag.Code
	}{
		{"inside", linkRootName, inJunction, map[string]string{j: junctionReal}, nil},
		{"inside, nested", linkRootName, nested, map[string]string{j: junctionReal}, nil},
		{"outside", linkRootName, inJunction, map[string]string{j: junctionOutside}, []diag.Code{diag.E3706.Def().Code}},
		{"outside, nested", linkRootName, nested, map[string]string{j: junctionOutside}, []diag.Code{diag.E3706.Def().Code}},
		{"dangling", linkRootName, inJunction, map[string]string{j: junctionGhost}, []diag.Code{diag.E3701.Def().Code}},
		{"root through a junction outside", rootJ, junctionPNG, map[string]string{j: junctionOutside}, []diag.Code{diag.E3706.Def().Code}},
		{"root through a junction inside", rootJ, junctionPNG, map[string]string{j: junctionReal}, nil},
		{"placeholder", linkRootName, placeholderName, map[string]string{junctionAssets + "/" + placeholderName: ""}, nil},
		{"placeholder through a junction", linkRootName, junctionName + "/" + placeholderName, map[string]string{j: junctionReal, junctionReal + "/" + placeholderName: ""}, nil},
	} {
		if got := assetCodes(t, junctionTree(c.root, c.icon, c.junctions)); !slices.Equal(got, c.want) {
			t.Errorf("%s%s: codes %v, want %v", junctionRule, c.name, got, c.want)
		}
	}
}

// WIRE.md §6.5, DECISIONS 352, 419: the re-read lists a junction only inside the bounds; a placeholder is a name.
func TestRereadJunction(t *testing.T) {
	fsys := junctionTree(linkRootName, junctionPNG, map[string]string{
		junctionAssets + "/in": junctionReal, junctionAssets + "/out": junctionOutside,
		junctionAssets + "/gone": junctionGhost, junctionAssets + "/" + placeholderName: "",
	})
	layout, ok := project.Place(&project.Project{}, junctionProj, project.Placement{FS: fsys}, diag.NewBag(nil, ""))
	if !ok {
		t.Fatal("layout")
	}
	got := listedNow(fsys, load.LinkBounds(fsys, layout), junctionAssets)
	slices.Sort(got)
	if want := []string{"in", placeholderName}; !slices.Equal(got, want) {
		t.Errorf("%slisted %v, want %v", junctionRule, got, want)
	}
}

// log-2026-09-29 M4 P14-r3, P14-r4, DECISIONS 419: a junction is a link to the real paths, its own
// name included; a dangling one leads through its text, a volume mount point never; a placeholder is its own name.
func TestRealPathsJunction(t *testing.T) {
	fsys := junctionTree(linkRootName, junctionPNG, map[string]string{
		"/p/a/j": junctionReal, "/p/alias": junctionReal, "/p/a/gone": "/p/real/new.json", "/p/a/" + placeholderName: "",
		"/p/a/vol": volumeTarget,
	})
	r := newRealPaths(fsys)
	for name, want := range map[string]string{ //canon:unordered each name is resolved alone
		"/p/a/j":                     junctionReal,
		"/p/a/j/" + junctionPNG:      junctionReal + "/" + junctionPNG,
		"/p/alias/x/y.json":          junctionReal + "/x/y.json",
		"/p/a/gone":                  "/p/real/new.json",
		"/p/a/" + placeholderName:    "/p/a/" + placeholderName,
		junctionAssets + "/new.json": junctionAssets + "/new.json",
		"/p/a/vol":                   "/p/a/vol",
		"/p/a/vol/x.json":            "/p/a/vol/x.json",
	} {
		if got := r.file(name); got != want {
			t.Errorf("%s%s resolves to %s, want %s", junctionRule, name, got, want)
		}
	}
}
