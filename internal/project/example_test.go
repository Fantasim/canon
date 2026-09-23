package project_test

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
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

// Paths resolve inside their root, moved by an override, or the project (WIRE.md §2).
func ExampleLayout_Resolve() {
	var set source.FileSet
	src, err := set.Add(project.FileName, "/law/project.canon", []byte(
		"project acme {\n  canon: \"0.1\"\n  roots {\n    resource: \"../Resource\"\n  }\n}\n"))
	if err != nil {
		fmt.Println(err)
		return
	}
	bag := diag.NewBag(&set, "")
	p, err := project.Load(src, bag)
	if err != nil {
		fmt.Println(err)
		return
	}
	l, _ := project.NewLayout(p, "/law", map[string]string{"resource": "fixtures/resource"}, bag)
	var out []string
	for _, written := range []string{"@resource/Server/../Server/x.h", "data/", "../pipeline/a.json", "../../x"} {
		if r, ok := l.Resolve(written, "items", source.Span{}, bag); ok {
			out = append(out, r.Display, r.Abs)
		}
	}
	fmt.Println(strings.Join(out, " "))
	fmt.Println(bag.Findings()[0].Message)
	// Output:
	// @resource/Server/x.h /law/fixtures/resource/Server/x.h items/data/ /law/items/data pipeline/a.json /law/pipeline/a.json
	// path ../../x: leaves the project
}
