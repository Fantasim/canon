package main

import (
	"fmt"
	"go/format"
	"slices"
	"strings"
)

// emit writes the structs as one gofmt-ed Go file.
func emit(structs []*structDef) ([]byte, error) {
	var b strings.Builder
	b.WriteString(fileHeader)
	if slices.ContainsFunc(structs, usesRaw) {
		b.WriteString(importJSON)
	}
	for _, sd := range structs {
		fmt.Fprintf(&b, fmtStruct, sd.name, sd.doc, sd.name)
		for _, f := range sd.fields {
			fmt.Fprintf(&b, fmtField, f.name, f.goType(), f.json, f.omit(), f.comment())
		}
		b.WriteString(endStruct)
	}
	src, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, fmt.Errorf("format: %w", err)
	}
	return src, nil
}

func usesRaw(sd *structDef) bool {
	return slices.ContainsFunc(sd.fields, func(f *field) bool {
		t := f.t
		for t.elem != nil {
			t = t.elem
		}
		return t.kind == tRaw
	})
}

// goType is the field's Go type: a member that may be absent is a pointer when it is a
// struct, or a scalar whose zero value the schema also admits (VIEWMODEL.md J3).
func (f *field) goType() string {
	s := f.t.String()
	if f.always {
		return s
	}
	if f.t.kind == tStruct || f.zeroOK && pointerable[f.t.kind] {
		return ptrPrefix + s
	}
	return s
}

// omit is the tag option of a member that may be absent: its zero value means absent.
func (f *field) omit() string {
	if f.always {
		return ""
	}
	return omitZero
}
