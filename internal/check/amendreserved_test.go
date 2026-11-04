package check_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const reservedBase = `package a

local record Row {
  code: String
  w: Int
}

local record Cfg {
  inner: table Row
}

local let tb: table Row = { a { code: "a", w: 0 } }
local let cfg: Cfg = { inner: { a { code: "a", w: 0 } } }
local let rows: [Row] keyed by code = [
  { code: "true", w: 0 },
  { code: "false", w: 0 },
  { code: "none", w: 0 },
  { code: "self", w: 0 }
]
`

// reservedAmends are amend blocks whose paths name none, true, false or self.
var reservedAmends = []struct{ name, block string }{
	{"a table entry added", "amend tb {\n  true: { code: \"true\", w: 1 }\n  none: { code: \"none\", w: 2 }\n}\n"},
	{"a table entry amended", "amend tb {\n  false.w: 3\n  self.w: 4\n}\n"},
	{"a nested table's entries", "amend cfg {\n  inner.true.w: 5\n  inner.self: { code: \"self\", w: 6 }\n}\n"},
	{"a String-keyed list's rows", "amend rows {\n  true.w: 7\n  false.w: 8\n  none: { code: \"none\", w: 9 }\n  self.w: 10\n}\n"},
}

// GRAMMAR.md §4.3 (table scope), §5.7, DECISIONS 305: an amend path may name none, true, false, self as a table or keyed-list key.
func TestAmendPathMayNameReservedKeys(t *testing.T) {
	for _, tc := range reservedAmends {
		set := &source.FileSet{}
		bags := check.Bags{builtPackage: diag.NewBag(set, builtPackage)}
		var files []*syntax.File
		for _, f := range [][2]string{{builtFile, reservedBase}, {layerFile, "package a\nlayer dev\n\n" + tc.block}} {
			src, err := set.Add(f[0], "/"+f[0], []byte(f[1]))
			if err != nil {
				t.Fatal(err)
			}
			files = append(files, syntax.Parse(src, syntax.FileSource, bags[builtPackage]))
		}
		check.Check(context.Background(), exampleProject(), files, bags, literalFolder{})
		if out := render(t, set, diag.NewBag(set, ""), bags); !strings.HasPrefix(out, noFindings) {
			t.Errorf("%s: want no finding:\n%s", tc.name, out)
		}
	}
}
