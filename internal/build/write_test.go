package build_test

import (
	"context"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
)

const tierSource = "/// A.\npackage a\n\n/// Tier.\nrecord Tier {\n  /// Weight.\n  weight: Int = 1\n}\n\n" +
	"/// Tiers.\nlet tiers: stable table Tier = { low {} }\n\nemit json { out: \"@out/\" }\n"

var _ build.WriteFS = mapFS{}

// failingFS fails the n-th rename (from 1), then works again.
type failingFS struct {
	mapFS
	renames, failAt int
}

var errRename = errors.New("rename failed")

func (f *failingFS) Rename(oldname, newname string) error {
	f.renames++
	if f.renames == f.failAt {
		return errRename
	}
	return f.mapFS.Rename(oldname, newname)
}

// readOnly is a project.FS without the writes of build.WriteFS.
type readOnly struct{ project.FS }

func tierTree() mapFS {
	return mapFS{
		"p/project.canon":  file(outProject),
		"p/a/a.canon":      file(tierSource),
		"p/out/tiers.json": file("{\"$schema\": \"a.tiers@00000000\", \"rows\": []}\n"),
		"p/b/b.canon":      file("/// B.\npackage b\n"),
		"p/b/canon.lock":   file("# canon.lock v1\n"),
		"p/b/c/c.canon":    file("/// C.\npackage b.c\n"),
		"p/b/c/canon.lock": file("# canon.lock v1\n"),
	}
}

func snapshot(m mapFS) map[string]string {
	out := map[string]string{}
	//canon:unordered each file is copied under its own name
	for name, f := range m {
		out[name] = string(f.Data)
	}
	return out
}

// CLI.md §3.4, API.md N11: a failed rename restores every file and leaves no temporary file.
func TestWriteAllOrNothing(t *testing.T) {
	fsys := &failingFS{mapFS: tierTree(), failAt: 2}
	before := snapshot(fsys.mapFS)
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Build(context.Background(), build.BuildOptions{}); !errors.Is(err, errRename) {
		t.Fatalf("build: %v", err)
	}
	if after := snapshot(fsys.mapFS); !maps.Equal(before, after) {
		t.Errorf("files changed:\n%v\nwant\n%v", after, before)
	}
	fsys.failAt = 0
	res, err := p.Build(context.Background(), build.BuildOptions{})
	if err != nil || len(res.Outputs) != 1 || len(res.Locks) != 3 || fsys.mapFS["p/a/canon.lock"] == nil {
		t.Errorf("second build: %v, %+v", err, res)
	}
}

// API.md §13.1, LOCK.md §5: in Check mode nothing is written and what would change is stale.
func TestCheckMode(t *testing.T) {
	fsys := tierTree()
	before := snapshot(fsys)
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Build(context.Background(), build.BuildOptions{Check: true})
	if err != nil || !res.Stale || res.Outputs[0].Status != build.StatusStale || res.Locks[0].Status != build.StatusStale {
		t.Fatalf("check: %v, %+v", err, res)
	}
	if !maps.Equal(before, snapshot(fsys)) {
		t.Error("Check mode wrote")
	}
	if _, err := p.Build(context.Background(), build.BuildOptions{}); err != nil {
		t.Fatal(err)
	}
	if res, err := p.Build(context.Background(), build.BuildOptions{Check: true}); err != nil || res.Stale {
		t.Errorf("after a build: %v, %+v", err, res)
	}
	ro, err := build.Open(readOnly{fsys}, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	fsys["p/a/a.canon"] = file(tierSource + "\n/// W.\nlet w: Int = 1\n")
	if _, err := ro.Build(context.Background(), build.BuildOptions{}); !errors.Is(err, build.ErrReadOnly) {
		t.Errorf("read-only: %v", err)
	}
}

// API.md S3, DECISIONS 196: the revision hashes the lock of every package directory, whatever the
// selection, so Revision equals the revision a check reports.
func TestRevisionHashesLocks(t *testing.T) {
	fsys := tierTree()
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rev, err := p.Revision(ctx)
	res, cerr := p.Check(ctx, []string{"a"})
	if err != nil || cerr != nil || res.Revision != rev {
		t.Fatalf("revision %s, check %v %v", rev, res, cerr)
	}
	fsys["p/b/c/canon.lock"] = file("# canon.lock v1\ntable  b.c.t  x\n")
	again, err := p.Revision(ctx)
	res, cerr = p.Check(ctx, []string{"a"})
	if err != nil || cerr != nil || again == rev || res.Revision != again {
		t.Errorf("after a lock changed: %s, %v", again, res)
	}
	delete(fsys, "p/b/c/canon.lock")
	if gone, err := p.Revision(ctx); err != nil || gone == again {
		t.Errorf("after a lock went: %s", gone)
	}
}

// linkErrorFS is mapFS whose Rename fails as a real cross-device rename does: with an
// *os.LinkError (not a *fs.PathError), both its own paths absolute.
type linkErrorFS struct{ mapFS }

func (f linkErrorFS) Rename(oldname, newname string) error {
	return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: os.ErrPermission}
}

// DECISIONS 201: a rename failure's *os.LinkError also names the display path; its own two
// absolute paths never appear.
func TestWriteRenameLinkErrorNamesDisplayPath(t *testing.T) {
	fsys := linkErrorFS{tierTree()}
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Build(context.Background(), build.BuildOptions{})
	if err == nil {
		t.Fatal("no error")
	}
	if msg := err.Error(); !strings.Contains(msg, "@out/tiers.json: rename: ") || strings.Contains(msg, "/p/out") {
		t.Errorf("build error = %q", msg)
	}
}

func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	//canon:unordered each file is written under its own name
	for name, text := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// CLI.md §3.4: build.OS writes on disk; a directory it cannot create fails with nothing written.
func TestOSWrites(t *testing.T) {
	for _, broken := range []bool{false, true} {
		dir := t.TempDir()
		files := map[string]string{"project.canon": outProject, "a/a.canon": tierSource}
		if broken {
			files["out"] = "a file where the output directory goes"
		}
		writeTree(t, dir, files)
		p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		_, err = p.Build(context.Background(), build.BuildOptions{})
		_, lockErr := os.Stat(filepath.Join(dir, "a", "canon.lock"))
		_, outErr := os.Stat(filepath.Join(dir, "out", "tiers.json"))
		if broken != (err != nil) || broken != (lockErr != nil) || broken != (outErr != nil) {
			t.Errorf("broken %t: %v, lock %v, output %v", broken, err, lockErr, outErr)
		}
	}
}

// DECISIONS 201: a write error names the display path, its op and cause kept; the absolute
// temporary path never appears.
func TestOSWriteErrorNamesDisplayPath(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"project.canon": outProject, "a/a.canon": jsonSource})
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(out, 0o750) })
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Build(context.Background(), build.BuildOptions{})
	if err == nil {
		t.Fatal("build over a read-only output directory: no error")
	}
	if msg := err.Error(); !strings.Contains(msg, "@out/v.json: open: ") || strings.Contains(msg, dir) {
		t.Errorf("write error = %q, dir %q", msg, dir)
	}
}

// DECISIONS 201, CLI.md §3.4: an unreadable source or canon.lock names its project-relative display path, never the absolute one, for both Build and Check.
func TestUnreadableFileNamesDisplayPath(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	for _, c := range []struct {
		name, display string
		unready       func(t *testing.T, dir string)
	}{
		{"source", "a/a.canon", func(t *testing.T, dir string) {
			t.Helper()
			path := filepath.Join(dir, "a", "a.canon")
			if err := os.Chmod(path, 0o000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
		}},
		{"lock", "a/canon.lock", func(t *testing.T, dir string) {
			t.Helper()
			if err := os.Mkdir(filepath.Join(dir, "a", "canon.lock"), 0o750); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, map[string]string{"project.canon": outProject, "a/a.canon": jsonSource})
			c.unready(t, dir)
			p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
			if err != nil {
				t.Fatal(err)
			}
			for _, call := range []struct {
				name string
				run  func() error
			}{
				{"build", func() error { _, err := p.Build(context.Background(), build.BuildOptions{}); return err }},
				{"check", func() error { _, err := p.Check(context.Background(), nil); return err }},
			} {
				t.Run(call.name, func(t *testing.T) {
					err := call.run()
					if err == nil {
						t.Fatal("no error")
					}
					if msg := err.Error(); !strings.Contains(msg, c.display+": ") || strings.Contains(msg, dir) {
						t.Errorf("%s error = %q, dir %q", call.name, msg, dir)
					}
				})
			}
		})
	}
}

// DECISIONS 201, CLI.md §2.1: an unreadable project directory is named ".", by Build and Check.
func TestUnreadableProjectDirNamesDot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"project.canon": outProject, "a/a.canon": jsonSource})
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o750) })
	for _, call := range []struct {
		name string
		run  func() error
	}{
		{"build", func() error { _, err := p.Build(context.Background(), build.BuildOptions{}); return err }},
		{"check", func() error { _, err := p.Check(context.Background(), nil); return err }},
	} {
		t.Run(call.name, func(t *testing.T) {
			err := call.run()
			if !errors.Is(err, fs.ErrPermission) {
				t.Fatalf("%s error = %v, want fs.ErrPermission", call.name, err)
			}
			if msg := err.Error(); !strings.HasPrefix(msg, ".: ") || strings.Contains(msg, dir) {
				t.Errorf("%s error = %q, dir %q", call.name, msg, dir)
			}
		})
	}
}

// failingOS is build.OS whose n-th rename fails.
type failingOS struct {
	build.WriteFS
	renames, failAt int
}

func (f *failingOS) Rename(oldname, newname string) error {
	f.renames++
	if f.renames == f.failAt {
		return errRename
	}
	return f.WriteFS.Rename(oldname, newname)
}

// API.md N11, DECISIONS 144: a failed write removes the directories it created; a replaced file
// keeps its mode.
func TestOSRollbackAndModes(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"project.canon": outProject, "a/a.canon": tierSource})
	fsys := &failingOS{WriteFS: build.OS(), failAt: 2}
	p, err := build.Open(fsys, filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Build(context.Background(), build.BuildOptions{}); !errors.Is(err, errRename) {
		t.Fatalf("build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "out")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("out/ left behind: %v", err)
	}
	q, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.Build(context.Background(), build.BuildOptions{}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out", "tiers.json")
	if err := os.Chmod(out, 0o644); err != nil {
		t.Fatal(err)
	}
	writeTree(t, dir, map[string]string{"a/a.canon": strings.Replace(tierSource, "low {}", "low { weight: 2 }", 1)})
	if _, err := q.Build(context.Background(), build.BuildOptions{}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(out)
	lock, lerr := os.Stat(filepath.Join(dir, "a", "canon.lock"))
	if err != nil || info.Mode().Perm() != 0o644 || lerr != nil || lock.Mode().Perm() != 0o600 {
		t.Errorf("modes: %v %v, %v %v", info, err, lock, lerr)
	}
}
