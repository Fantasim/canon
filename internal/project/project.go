package project

import (
	"cmp"
	"slices"
	"strconv"

	"github.com/fantasim/canonlang/internal/source"
)

// Project is a checked project.canon (GRAMMAR.md §7.1); Roots and GoModules are in name order.
type Project struct {
	Name      string
	Canon     Version
	Roots     []Root
	Languages []string // the first is the source language (SPEC §17)
	Studio    Package
	Budget    int64 // 0 when not declared: the evaluator's default applies (IMPLEMENTATION-PLAN §12.4)
	GoModules []GoModule
}

// Version is the language version `canon` states, "MAJOR.MINOR" (GRAMMAR.md §7.1).
type Version struct {
	Major, Minor int
}

// Root is a named root: its name and its path as written, relative to the project directory.
type Root struct {
	Name string
	Path string
	Span source.Span
}

// Package is a package path the project names, with the span that names it; Path "" is none.
type Package struct {
	Path string
	Span source.Span
}

// GoModule is the Go import path of a root's directory (CODEGEN.md §2.8).
type GoModule struct {
	Root   string
	Module string
	Span   source.Span
}

// New is the project `name` states with every other key at its default (GRAMMAR.md §7.1).
func New(name string, canon Version) *Project {
	return &Project{Name: name, Canon: canon, Languages: []string{DefaultLanguage}}
}

// String is the version as project.canon writes it.
func (v Version) String() string {
	return strconv.Itoa(v.Major) + versionSep + strconv.Itoa(v.Minor)
}

// Supported reports whether this compiler reads v: same major, a known minor (NFR-03).
func (v Version) Supported() bool {
	return slices.Contains(supported[:], v)
}

// SupportedVersions lists the versions this compiler reads, in order (E1001).
func SupportedVersions() []Version {
	return slices.Clone(supported[:])
}

// SourceLanguage is the first of Languages (SPEC §17).
func (p *Project) SourceLanguage() string {
	if len(p.Languages) == 0 {
		return DefaultLanguage
	}
	return p.Languages[0]
}

// Root is the root declared under name.
func (p *Project) Root(name string) (Root, bool) {
	i, found := slices.BinarySearchFunc(p.Roots, name, func(r Root, n string) int { return cmp.Compare(r.Name, n) })
	if !found {
		return Root{}, false
	}
	return p.Roots[i], true
}

// GoModule is the Go module path mapped to root name (CODEGEN.md §2.8).
func (p *Project) GoModule(root string) (GoModule, bool) {
	i, found := slices.BinarySearchFunc(p.GoModules, root, func(g GoModule, n string) int { return cmp.Compare(g.Root, n) })
	if !found {
		return GoModule{}, false
	}
	return p.GoModules[i], true
}
