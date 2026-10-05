package ir_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// fingerprintedSource is a data-mode cpp emit of a define record value no json emit writes.
const fingerprintedSource = `package a

/// A rule.
record Rule {
  /// A define.
  it: Define
}

/// The rule.
let rule: Rule = { it: { value: 3 } }

emit cpp { out: "@features/a", namespace: "a", mode: data }
`

// Decisions 126, 194: a data-mode code emit's value carries its fingerprint, which a define record has none of, so it is E8012 `define` there. Since DECISIONS 320 made cpp and go embedded E8019 `unbuilt`, no program shows it alone (a data value no json emit writes is E8153 too, one a json emit writes E8151), so this case checks it beside E8153 rather than as a findings fixture.
func TestFingerprintedDefineInDataMode(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(fingerprintedSource))
	w.calls = w.fixtureCalls
	w.build(t)
	out := w.findings(t)
	for _, code := range []diag.Code{diag.E8012.Def().Code, diag.E8153.Def().Code} {
		if !strings.Contains(out, "["+string(code)+"]") {
			t.Errorf("want %s:\n%s", code, out)
		}
	}
}
