package ir_test

import (
	"strings"
	"testing"
)

// TestCppMacroRenamedIsClean is CODEGEN.md §3.5: @cpp(name:) is the way out, and another case is no macro; go and ts emits take the names.
func TestCppMacroRenamedIsClean(t *testing.T) {
	for name, src := range map[string]string{ //canon:unordered each case alone
		"renamed constant": `package a

/// The largest signed 32-bit value.
@cpp(name: "kInt32Max")
const INT32_MAX = 2147483647

emit cpp { out: "@features/a", namespace: "a", mode: types }
`,
		"renamed enum member": `package a

/// A read result.
enum Read { EOF @cpp(name: "End"), ok }

emit cpp { out: "@features/a", namespace: "a", mode: data }
`,
		"go and ts emits": `package a

/// The largest unsigned 32-bit value.
const UINT32_MAX = 4294967295

/// A read result.
enum Read { EOF, ok }

emit go { out: "@features/a", package: "a" }
emit ts { out: "@features/a/a.ts" }
`,
		"macro spelled in another case": `package a

/// A stream.
record Stream {
  /// Its errno, stored as errno_ and read by GetErrno.
  errno: Int
  /// Its end.
  Eof: Int
}

emit cpp { out: "@features/a", namespace: "a", mode: data }
`,
	} {
		w := newWorld(t)
		w.add(t, "a/a.canon", []byte(src))
		w.calls = w.fixtureCalls
		w.build(t)
		if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
			t.Errorf("%s must be clean:\n%s", name, out)
		}
	}
}
