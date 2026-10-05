package ir_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// reStageE is a rendered finding of stage E: CODEGEN.md's and WIRE.md's E8xxx, CONFORMANCE.md's E9xxx.
var reStageE = regexp.MustCompile(`(?:error|warning)\[[EW][89][0-9]{3}\]`)

// TestBrokenDeclarationsReachNoStageE is DECISIONS 213 (a broken declaration, and everything naming it, never enters the emit IR) and 209 (a broken check breaks its record), per kind of declaration stage E judges: each program has one static error in a declaration that a stage-E rule would refuse too (a package fn E8013, a method E8014, a let E8015, a const E8005, a variant, a record and a record naming it E8012), and stage E reports nothing.
func TestBrokenDeclarationsReachNoStageE(t *testing.T) {
	cases := []struct{ name, src string }{
		{"package fn in data mode", `package a

/// The answer.
export fn answer() -> Int { return zzUnknown }

emit go { out: "@features/a", package: "a", mode: data }
`},
		{"method in types mode", `package a

/// A potion.
record Potion {
  /// Its heal.
  heal: Int
  /// Its double.
  export fn twice() -> Int { return zzUnknown }
}

emit cpp { out: "@features/a", namespace: "a", mode: types }
`},
		{"Int let in data mode", `package a

/// A cap.
let cap: Int = zzUnknown

emit go { out: "@features/a", package: "a", mode: data }
`},
		{"const named like another", `package a

/// The version.
const VERSION = 7
/// The version again.
const version = zzUnknown

emit go { out: "@features/a", package: "a" }
`},
		{"variant with a function field", `package a

/// An event.
variant Event {
  /// A spawn.
  spawn { zzFn: fn(Int) -> Int }
  /// A despawn.
  despawn
}

emit go { out: "@features/a", package: "a" }
`},
		{"record with a Range field and a broken check", `package a

/// A window.
record Window {
  /// Its span.
  span: Range
  check zzUnknown else "never"
}

emit go { out: "@features/a", package: "a" }
`},
		{"record naming a broken record", `package a

/// A time of day.
record TimeOfDay {
  /// A function.
  zzFn: fn(Int) -> Int
}

/// A shift.
record Shift {
  /// Its start.
  start: TimeOfDay
}

/// The shifts.
let shifts: [Shift] = []

emit go { out: "@features/a", package: "a" }
emit json { out: "out/" }
`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			w.add(t, "a/a.canon", []byte(c.src))
			w.calls = w.fixtureCalls
			w.build(t)
			out := w.findings(t)
			if !reCheckError.MatchString(out) || reSyntaxError.MatchString(out) {
				t.Fatalf("the program must have a phase-2 error and no syntax error:\n%s", out)
			}
			if reStageE.MatchString(out) {
				t.Errorf("stage E judged a broken declaration:\n%s", out)
			}
		})
	}
}

// TestModeRefusedEmitKeepsItsOutput is DECISIONS 213: an emit whose mode check refused keeps its out, directory and imports in the rules that do not depend on the mode, so it still meets another emit writing its directory (E8008, at the sound one, named second), still needs its imported package's go emit (E8004), and a ts one still refuses input fields (E8104).
func TestModeRefusedEmitKeepsItsOutput(t *testing.T) {
	requireFindings(t, []string{"c/c.canon", `package c

/// A colour.
enum Tone { red, blue }
`, "e/e.canon", `package e

import c { Tone }

/// A badge.
record Badge {
  /// Its colour.
  tone: Tone
}

emit go { out: "@features/e", package: "e", mode: zzmode }
`, "a/a.canon", `package a

emit go { out: "@features/shared", package: "shared", mode: zzmode }
`, "b/b.canon", `package b

emit go { out: "@features/shared", package: "shared" }
`, "t/t.canon", `package t

/// The service settings.
record Settings {
  /// The key of the API.
  apiKey: input String? from env "API_KEY"
}

/// The settings.
let settings: Settings = {}

emit ts { out: "out/t.ts", mode: zzmode }
`}, []wantAt{
		{string(diag.E8009.Def().Code), "a/a.canon"},
		{string(diag.E8008.Def().Code), "b/b.canon"},
		{string(diag.E8004.Def().Code), "e/e.canon"},
		{string(diag.E8104.Def().Code), "t/t.canon"},
	})
}

// wantAt is a finding a program must report: its code, in a file.
type wantAt struct{ code, file string }

// requireFindings builds files (name, source pairs) and requires each wanted finding.
func requireFindings(t *testing.T, files []string, wants []wantAt) {
	t.Helper()
	w := newWorld(t)
	for i := 0; i+1 < len(files); i += 2 {
		w.add(t, files[i], []byte(files[i+1]))
	}
	w.calls = w.fixtureCalls
	w.build(t)
	out := w.findings(t)
	for _, want := range wants {
		if at := "[" + want.code + "]  " + want.file; !strings.Contains(out, at) {
			t.Errorf("want %s:\n%s", at, out)
		}
	}
}

var (
	// reCheckError is a rendered error of phase 2, a TYPES.md code (E2xxx, E3xxx).
	reCheckError = regexp.MustCompile(`error\[E[23][0-9]{3}\]`)
	// reSyntaxError is a rendered GRAMMAR.md error (E1xxx): a fixture that does not parse proves nothing.
	reSyntaxError = regexp.MustCompile(`error\[E1[0-9]{3}\]`)
)
