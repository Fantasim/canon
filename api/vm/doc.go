// Package vm holds the view-model structs that internal/views builds, generated into vm.gen.go
// from spec/viewmodel.schema.json by `go run ./api/vm/internal/vmgen` (API.md R10, make vm-check).
// A oneOf of objects is one struct with its branches' members, each branch's order kept (J2).
// An omitzero member may be absent, its zero value meaning absent: a pointer where the schema
// admits the zero value too, a nil slice or map (an empty non-nil one is present). Builders keep
// required slices and maps non-nil and apply J3's omissions; json.Marshal is not the J1 writer
// (compact, nil as null). encoding/json decodes member names case-insensitively and takes null
// for any pointer, slice or map, required or not: the strict check is ViewModel.Decode's.
// Number carries J10 numbers, Scalar a string or an integer, TextRef a J9 text reference, and
// json.RawMessage a J10 value.
package vm
