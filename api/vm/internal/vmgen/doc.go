// Command vmgen generates api/vm/vm.gen.go, the view-model Go structs, from
// spec/viewmodel.schema.json (API.md R10). Each object schema becomes a struct whose fields
// follow its member order (VIEWMODEL.md J2); a oneOf of objects becomes one struct holding
// the union of its branches' members. vmgen refuses, writing nothing, a schema whose branches
// disagree on a member's type or order, an object not closed (additionalProperties false), a
// keyword it does not know, an alias definition, or a definition that reaches itself.
//
// Run from the repository root (make vm-check diffs its output against the committed file):
//
//	go run ./api/vm/internal/vmgen [-schema spec/viewmodel.schema.json] [-out api/vm]
package main
