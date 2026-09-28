package progen

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/project"
)

// Project is a Canon project held in memory: its files and symbolic links by project-relative
// '/' path. The build reads it as the directory projectDir of a file system holding nothing else.
type Project struct {
	files map[string][]byte
	links map[string]string // a link's target, as the link holds it
}

// NewProject is an empty project.
func NewProject() *Project { return &Project{files: map[string][]byte{}, links: map[string]string{}} }

// LoadDir reads every file under dir into a project, skipping the directories whose name skip
// lists (such as examples' generated expected/ trees).
func LoadDir(dir string, skip ...string) (*Project, error) {
	p := NewProject()
	fsys := os.DirFS(dir)
	err := fs.WalkDir(fsys, currentDir, func(name string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && name != currentDir && slices.Contains(skip, d.Name()):
			return fs.SkipDir
		case d.IsDir():
			return nil
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("progen: %w", err)
		}
		p.files[name] = data
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("progen: %w", err)
	}
	return p, nil
}

// Set writes a file.
func (p *Project) Set(name string, data []byte) { p.files[name] = data }

// Remove deletes a file.
func (p *Project) Remove(name string) { delete(p.files, name) }

// Get is a file's content.
func (p *Project) Get(name string) ([]byte, bool) {
	data, ok := p.files[name]
	return data, ok
}

// Names are the project's file paths in byte order.
func (p *Project) Names() []string { return slices.Sorted(maps.Keys(p.files)) }

// Link makes name a symbolic link to target.
func (p *Project) Link(name, target string) { p.links[name] = target }

// linkNames are the project's symbolic link paths in byte order.
func (p *Project) linkNames() []string { return slices.Sorted(maps.Keys(p.links)) }

// Clone is a copy that shares no map with p; contents are never written in place.
func (p *Project) Clone() *Project {
	return &Project{files: maps.Clone(p.files), links: maps.Clone(p.links)}
}

// fsys is the project as a file system rooted at "/", the project at projectDir.
func (p *Project) fsys() memFS {
	m := fstest.MapFS{}
	prefix := strings.TrimPrefix(projectDir, rootDir)
	for name, data := range p.files { //canon:unordered building a map from a map
		m[path.Join(prefix, name)] = &fstest.MapFile{Data: data}
	}
	for name, target := range p.links { //canon:unordered building a map from a map
		m[path.Join(prefix, name)] = &fstest.MapFile{Data: []byte(target), Mode: fs.ModeSymlink}
	}
	return memFS(m)
}

// memFS serves absolute '/' paths from a MapFS, as project.FS asks.
type memFS fstest.MapFS

func rel(name string) string {
	if name == rootDir {
		return currentDir
	}
	return strings.TrimPrefix(name, rootDir)
}

func (m memFS) ReadFile(name string) ([]byte, error) {
	return wrap(fstest.MapFS(m).ReadFile(rel(name)))
}

func (m memFS) Stat(name string) (fs.FileInfo, error) {
	return wrap(fstest.MapFS(m).Stat(rel(name)))
}

func (m memFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return wrap(fstest.MapFS(m).ReadDir(rel(name)))
}

func wrap[T any](v T, err error) (T, error) {
	if err != nil {
		return v, fmt.Errorf("progen: %w", err)
	}
	return v, nil
}

// EvalSymlinks is name with every link on its way followed: the optional capability through
// which load.dir follows links (project.EvalSymlinks); an error when the result does not exist.
func (m memFS) EvalSymlinks(name string) (string, error) {
	cur, parts := rootDir, strings.Split(rel(name), rootDir)
	for hops := 0; len(parts) > 0; {
		next := path.Join(cur, parts[0])
		parts = parts[1:]
		f, ok := m[rel(next)]
		if !ok || f.Mode&fs.ModeSymlink == 0 {
			cur = next
			continue
		}
		if hops++; hops > project.MaxSymlinkHops {
			return "", fmt.Errorf("%w: %s", project.ErrSymlinkLoop, name)
		}
		target := string(f.Data)
		if !path.IsAbs(target) {
			target = path.Join(cur, target)
		}
		parts = append(strings.Split(rel(target), rootDir), parts...)
		cur = rootDir
	}
	if _, err := m.Stat(cur); err != nil {
		return "", err
	}
	return cur, nil
}
