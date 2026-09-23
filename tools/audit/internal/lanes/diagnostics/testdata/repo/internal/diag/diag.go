// Package diag is the registry: the one package that may name codes and hold their texts.
package diag

type Finding struct {
	Code    string
	Message string
}

type Related struct{ Note string }

type Bag struct{ Findings []Finding }

type Builder struct{ f Finding }

func (b *Builder) Check(name string) *Builder { return b }

func (b *Builder) Report(bag *Bag) { bag.Findings = append(bag.Findings, b.f) }

type codeE1001 struct{}

var E1001 codeE1001

func (codeE1001) At(span int, version string, supported []string) *Builder {
	return &Builder{f: Finding{Code: "E1001", Message: "project requires Canon " + version}}
}

type codeE1002 struct{}

var E1002 codeE1002

func (codeE1002) At(span int, key string) *Builder {
	return &Builder{f: Finding{Code: "E1002", Message: "unknown project key " + key}}
}

type codeE1003 struct{}

var E1003 codeE1003

func (codeE1003) At(span int, dir string) *Builder { return &Builder{} }

type codeW1001 struct{}

var W1001 codeW1001

func (codeW1001) At(span int) *Builder {
	return &Builder{f: Finding{Code: "W1001", Message: "doc comment is not attached to anything"}}
}
