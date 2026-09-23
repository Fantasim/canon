package project_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/project"
)

// A project states its name and version; every other key has its default (GRAMMAR.md §7.1).
func ExampleNew() {
	p := project.New("sovereign", project.Version{Major: 0, Minor: 1})
	p.Roots = []project.Root{{Name: "resource", Path: "../../../Resource"}, {Name: "services", Path: "../.."}}
	p.GoModules = []project.GoModule{{Root: "services", Module: "gitlab.com/sovereign15"}}
	r, _ := p.Root("resource")
	g, _ := p.GoModule("services")
	fmt.Println(p.Canon, p.Canon.Supported(), p.SourceLanguage(), r.Path, g.Module, p.Budget)
	// Output: 0.1 true en ../../../Resource gitlab.com/sovereign15 0
}
