package main

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// buildSpec is the whole project's shape and its two fixed field tables, computed once per run.
type buildSpec struct {
	n          int
	model      *genModel
	shared     sharedSpecs
	caseFields []fieldSpec
}

// definesHeader is items/defines/categories.h: one `#define NAME N` line per category, in order.
func definesHeader(categories []string) []byte {
	var b strings.Builder
	b.WriteString("// Written by internal/testkit/cmd/benchgen.\n")
	for i, name := range categories {
		fmt.Fprintf(&b, "#define %s %d\n", name, i+1)
	}
	return []byte(b.String())
}

// generate writes the whole benchmark project of n entries under out, seeded from seed.
func generate(seed uint64, out string, n int) error {
	r := progen.NewRand(seed)
	spec := buildSpec{n: n, model: buildModel(n), shared: buildSharedSpecs(), caseFields: caseFieldSpecs()}
	if err := writeStatics(out, r, spec); err != nil {
		return err
	}
	for i := range n {
		it := generateItem(r, i, spec.model, spec.shared, spec.caseFields)
		if err := writeEntryFiles(out, it); err != nil {
			return err
		}
	}
	return nil
}

// writeStatics writes the 5 files that do not repeat per entry, in a fixed order.
func writeStatics(out string, r *progen.Rand, spec buildSpec) error {
	if err := writeFile(out, "project.canon", projectCanon()); err != nil {
		return err
	}
	if err := writeFile(out, "monster/monster.canon", monsterCanon(r, spec.model)); err != nil {
		return err
	}
	if err := writeFile(out, "items/item.canon", itemCanon(spec.n, spec.model, spec.shared, spec.caseFields)); err != nil {
		return err
	}
	if err := writeFile(out, "items/defines/categories.h", definesHeader(spec.model.categories)); err != nil {
		return err
	}
	return writeFile(out, "twin/twin.canon", twinCanon())
}

// writeEntryFiles writes one item's entry file, icon placeholder and JSON twin.
func writeEntryFiles(out string, it *itemData) error {
	if err := writeFile(out, "items/"+entryPath(it), entryCanon(it)); err != nil {
		return err
	}
	if err := writeFile(out, "items/icons/"+it.code+iconExt, nil); err != nil {
		return err
	}
	return writeFile(out, "twin/data/"+it.code+ir.JSONExt, itemJSON(it))
}
