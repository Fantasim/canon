package project_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/project"
)

// GRAMMAR.md §7.1, NFR-03: this compiler reads 0.1 only; another minor or major is refused.
func TestVersions(t *testing.T) {
	if got := project.SupportedVersions(); !slices.Equal(got, []project.Version{{Major: 0, Minor: 1}}) {
		t.Errorf("SupportedVersions() = %v", got)
	}
	for _, v := range []project.Version{{Major: 0, Minor: 2}, {Major: 1, Minor: 1}, {Major: 0, Minor: 0}} {
		if v.Supported() {
			t.Errorf("%s is supported", v)
		}
	}
	if s := (project.Version{Major: 12, Minor: 30}).String(); s != "12.30" {
		t.Errorf("String() = %q", s)
	}
}

// GRAMMAR.md §7.1: lookups by root name; an empty Languages still has a source language.
func TestLookups(t *testing.T) {
	p := project.New("acme", project.Version{Minor: 1})
	p.Roots = []project.Root{{Name: "a", Path: "x"}, {Name: "b", Path: "y"}}
	p.GoModules = []project.GoModule{{Root: "b", Module: "example.com/b"}}
	p.Studio = project.Package{Path: "studio"}
	if r, ok := p.Root("b"); !ok || r.Path != "y" {
		t.Errorf("Root(b) = %v, %v", r, ok)
	}
	if _, ok := p.Root("c"); ok {
		t.Error("Root(c) found")
	}
	if _, ok := p.GoModule("a"); ok {
		t.Error("GoModule(a) found")
	}
	p.Languages = nil
	if p.SourceLanguage() != project.DefaultLanguage || p.Studio.Path != "studio" || p.Name != "acme" {
		t.Errorf("project %+v", p)
	}
}
