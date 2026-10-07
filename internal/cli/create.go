package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/project"
)

// runInit is `canon init [--name <project>]`, never replacing a project.canon (CLI.md §3.1).
func runInit(inv *invocation) int {
	if len(inv.args) > 0 {
		return inv.fail(fmt.Errorf(fmtArgs, cmdInit, errNoArgs))
	}
	dir := inv.abs(inv.opt.project)
	name := inv.opt.name
	if name == "" {
		name = filepath.Base(dir)
	}
	if !project.IsIdent(name) {
		return inv.fail(fmt.Errorf(fmtQuoted, name, errBadName))
	}
	versions := project.SupportedVersions()
	text := fmt.Sprintf(initFormat, name, versions[len(versions)-1])
	err := inDir(dir, func(root *os.Root) error {
		if err := createFile(root, project.FileName, text); err != nil {
			return err
		}
		return ignoreCache(root)
	})
	if err != nil {
		return inv.fail(err)
	}
	return exitOK
}

// runNew is `canon new <package>`: its directory and `<last segment>.canon` (CLI.md §3.2).
func runNew(inv *invocation) int {
	if len(inv.args) != 1 {
		return inv.fail(fmt.Errorf(fmtArgs, cmdNew, errOneArg))
	}
	name := inv.args[0]
	if !project.IsPackageName(name) {
		return inv.fail(fmt.Errorf(fmtQuoted, name, errBadPackage))
	}
	p, err := inv.openProject()
	if err != nil {
		return inv.fail(err)
	}
	root := p.Root()
	if err := p.Close(); err != nil {
		return inv.fail(err)
	}
	segs := strings.Split(name, dotSep)
	dir := strings.Join(segs, pathSep)
	err = inDir(filepath.FromSlash(root), func(r *os.Root) error {
		if err := r.MkdirAll(dir, dirPerm); err != nil {
			return fmt.Errorf(fmtWrap, err)
		}
		return createFile(r, dir+pathSep+segs[len(segs)-1]+project.SourceExt, fmt.Sprintf(newFormat, name, name))
	})
	if err != nil {
		return inv.fail(err)
	}
	return exitOK
}

// inDir runs write with the files of dir, which it cannot leave.
func inDir(dir string, write func(*os.Root) error) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	return errors.Join(write(root), root.Close())
}

// createFile writes a new file, named relative to root as CLI.md §2.1 shows it; never replaces.
func createFile(root *os.Root, name, text string) error {
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePerm)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf(fmtArgs, name, errExists)
	}
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	_, werr := f.WriteString(text)
	if err := errors.Join(werr, f.Close()); err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	return nil
}

// ignoreCache adds the `.canon/` and `project.local.canon` lines to .gitignore unless there (CLI.md §3.1).
func ignoreCache(root *os.Root) error {
	data, err := root.ReadFile(gitignoreFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf(fmtWrap, err)
	}
	text := string(data)
	lines := strings.Split(text, lineBreak)
	added := text
	for _, line := range [...]string{cacheIgnore, project.LocalFileName} {
		if slices.Contains(lines, line) {
			continue
		}
		if added != "" && !strings.HasSuffix(added, lineBreak) {
			added += lineBreak
		}
		added += line + lineBreak
	}
	if added == text {
		return nil
	}
	if err := root.WriteFile(gitignoreFile, []byte(added), filePerm); err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	return nil
}
