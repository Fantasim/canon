package canon_test

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

// editLaw is the project the edit tests write: a holds a table, a stable table, settings with
// defaults, a check and a plain list of table entries; b imports a; c is unrelated.
var editLaw = map[string]string{
	"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
	"a/a.canon": `/// A.
package a

/// A status.
record Status {
  /// Its label.
  label: String(1..)
  /// Its weight.
  weight: Int = 0
  /// Where it leads.
  next: [ref statuses] = []

  check weight >= 0 else "weight must not be negative"
}

/// A code.
record Code {
  /// Its label.
  label: String
}

/// Settings.
record Config {
  /// A port.
  port: Int = 8765
  /// A name.
  name: String = "x"
}

/// Statuses.
let statuses: table Status = {
  open { label: "Open", next: [done] }
  done { label: "Done" }
}

/// Codes, never renamed.
let codes: stable table Code = {
  first { label: "First" }
}

/// The settings.
let config: Config = {}

/// Two statuses, copied into a plain list.
let picked: [Status] = [statuses.open, statuses.done]

/// Twice the port, computed.
let twice: Int = config.port * 2
`,
	"a/canon.lock": "# canon.lock v1\ntable  a.codes  first\n",
	"b/b.canon": `/// B.
package b

import a

/// A box.
let box: Int = a.config.port
`,
	"c/c.canon": "/// C.\npackage c\n\n/// N.\nlet n: Int = 1\n",
}

// editPort is the edit most tests make: a's port, absent from its literal, set (API.md W7).
func editPort(base canon.Revision) canon.Edit {
	return canon.Edit{Base: base, Ops: []canon.Op{canon.Set("a:config.port", canon.Int(9000))}}
}

// openEdit opens editLaw with the changes given, and returns its project and file system.
func openEdit(t *testing.T, changes ...map[string]string) (*canon.Project, *memFS) {
	t.Helper()
	law := map[string]string{}
	for _, files := range append([]map[string]string{editLaw}, changes...) {
		for _, name := range slices.Sorted(maps.Keys(files)) {
			law[name] = files[name]
		}
	}
	opts := project(law)
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	m, _ := opts.FS.(*memFS)
	return p, m
}

// read is the content of the project file rel on m.
func read(t *testing.T, m *memFS, rel string) string {
	t.Helper()
	data, err := m.ReadFile("/law/" + rel)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// silent fails the test when an event comes within d: past W14's one-second cap, a watch that
// took an edit's own writes for an external change would have reported them (API.md W15).
func silent(t *testing.T, events <-chan canon.Event, d time.Duration, why string) {
	t.Helper()
	select {
	case ev := <-events:
		t.Errorf("%s: an event %s %v", why, ev.Cause, ev.Files)
	case <-time.After(d):
	}
}

// isErr reports err wrapping want, as the error type of API.md §15 T.
func isErr[T error](err, want error) (T, bool) {
	var typed T
	return typed, errors.Is(err, want) && errors.As(err, &typed)
}

// lines is text's lines holding sub.
func lines(text, sub string) []string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		if strings.Contains(line, sub) {
			out = append(out, line)
		}
	}
	return out
}
