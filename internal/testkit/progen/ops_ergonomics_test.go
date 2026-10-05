package progen_test

import "github.com/fantasim/canonlang/internal/diag"

// A table of one row, the collection the key operators test against.
const keyedRows = "local record ZzKeyRow {\n  n: Int\n}\n\n" +
	"local let zzKeyRows: table ZzKeyRow = {\n  zzOne { n: 1 }\n}\n\n"

// Operators on the ergonomics rules: tests, naming, C++ macro names, keys and optional defaults.
func ergonomicsOperators() []operator {
	return []operator{
		op(diag.E3026.Def().Code, "STDLIB.md §5 (a key on the left of in)", appendSite(keyedRows+
			"local let zzE3026: Bool = ", `"zzOne"`, " in zzKeyRows")),
		op(diag.E3027.Def().Code, "TYPES.md §4.1 (a name in scope used as a key)", appendSite(keyedRows+
			"local let zzKeyName: Int = 1\n\nlocal let zzE3027: Bool = zzKeyRows.hasKey(", "zzKeyName", ")")),
		op(diag.E5004.Def().Code, "EVALUATION.md §10.3 (fails names no check)", appendSite("test \"zz e5004\" {\n  expect 1 fails ", "zzNoSuchCheck", "\n}")),
		op(diag.E5005.Def().Code, "EVALUATION.md §10.1 (test name twice)", appendSite("test \"zz e5005\" {\n  expect true\n}\n\ntest ", `"zz e5005"`, " {\n  expect true\n}")),
		op(diag.W1003.Def().Code, "GRAMMAR.md §9.2 (const not UPPER_SNAKE)", appendSite("const ", "zzW1003", " = 7")),
		op(diag.W8006.Def().Code, "CODEGEN.md §3.5 (enum member is a platform macro)", withEmit("cpp", "/// Kind.\nenum ZzW8006 { ", "min", ", zzBig }")),
		op(diag.W3001.Def().Code, "TYPES.md §15 (optional field written = none)", appendSite("local record ZzW3001 {\n  /// N.\n  zzN: Int? = ", "none", "\n}")),
	}
}
