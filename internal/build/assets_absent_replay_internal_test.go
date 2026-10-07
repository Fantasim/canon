package build

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

const (
	flipDir     = "/client"
	flipProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    client: \"../client\"\n  }\n  optional_roots: [client]\n}\n"
	flipSource  = "/// A.\npackage a\n\n/// Item.\nrecord Item {\n  /// Icon.\n  icon: asset(\"@client/Icon\", ext: [png])\n}\n\n" +
		"/// Items.\nlet items: table Item = {}\n"
	flipEntry = "package a\n\nentry items.%s {\n  icon: \"%s.png\"\n}\n"
)

// hideFS is a project.FS that can make one directory vanish, as a root that is cloned or removed does.
type hideFS struct {
	project.FS
	hidden bool
}

func (h *hideFS) gone(name string) bool {
	return h.hidden && (name == flipDir || strings.HasPrefix(name, flipDir+pathSep))
}

func (h *hideFS) ReadFile(name string) ([]byte, error) {
	if h.gone(name) {
		return nil, fs.ErrNotExist
	}
	return h.FS.ReadFile(name)
}

func (h *hideFS) Stat(name string) (fs.FileInfo, error) {
	if h.gone(name) {
		return nil, fs.ErrNotExist
	}
	return h.FS.Stat(name)
}

func (h *hideFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if h.gone(name) {
		return nil, fs.ErrNotExist
	}
	return h.FS.ReadDir(name)
}

// TYPES.md §13.4: a warm analysis is a cold one as an asset root turns absent and present; the first flip replays nothing.
func TestAssetRootPresenceFlipsReplay(t *testing.T) {
	hide := &hideFS{FS: roFS{
		"p/project.canon": srcFile(flipProject), "p/a/a.canon": srcFile(flipSource),
		"p/a/sword.canon": srcFile(fmt.Sprintf(flipEntry, "sword", "sword")), "p/a/shield.canon": srcFile(fmt.Sprintf(flipEntry, "shield", "shield")),
		"client/Icon/sword.png": srcFile(""),
	}}
	z := &analyzer{fs: newEditFS(hide), dir: archiveRoot, cache: NewCache()}
	for _, st := range []struct {
		name   string
		hidden bool
		e3701  int
		e3705  int
		replay bool // stage B replays the entries
	}{
		{"present", false, 1, 0, false},
		{"present again", false, 1, 0, true},
		{"absent", true, 0, 2, false},
		{"absent again", true, 0, 2, true},
		{"present back", false, 1, 0, true},
	} {
		hide.hidden = st.hidden
		warm, cold := z.pair(t)
		same(t, st.name, warm, cold)
		missing, unchecked := diag.E3701.Def().Code, diag.E3705.Def().Code
		if n := countCode(warm, missing); n != st.e3701 {
			t.Errorf("%s: %d of %s, want %d", st.name, n, missing, st.e3701)
		}
		if n := countCode(warm, unchecked); n != st.e3705 {
			t.Errorf("%s: %d of %s, want %d", st.name, n, unchecked, st.e3705)
		}
		v, _ := replaysOf(warm)
		if (v > 0) != st.replay {
			t.Errorf("%s: stage B replayed %d entries, replay expected: %t", st.name, v, st.replay)
		}
	}
}
