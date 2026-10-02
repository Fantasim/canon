package cli

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

const helpFlag = "h"

// sentinels are the API's errors by Go name (API.md §15), as the guide names them.
var sentinels = map[string]error{
	"ErrNoProject": canon.ErrNoProject, "ErrProject": canon.ErrProject,
	"ErrUnsupportedVersion": canon.ErrUnsupportedVersion, "ErrUnknownPackage": canon.ErrUnknownPackage,
	"ErrUnknownLayer": canon.ErrUnknownLayer, "ErrBadPath": canon.ErrBadPath, "ErrNoPath": canon.ErrNoPath,
	"ErrAmbiguousPath": canon.ErrAmbiguousPath, "ErrNoValue": canon.ErrNoValue,
	"ErrInputField": canon.ErrInputField, "ErrBadOp": canon.ErrBadOp, "ErrBadValue": canon.ErrBadValue,
	"ErrKeyExists": canon.ErrKeyExists, "ErrStableKey": canon.ErrStableKey, "ErrNameClash": canon.ErrNameClash,
	"ErrNotEditable": canon.ErrNotEditable, "ErrStale": canon.ErrStale, "ErrRejected": canon.ErrRejected,
	"ErrNotCanonical": canon.ErrNotCanonical, "ErrPathCollision": canon.ErrPathCollision,
	"ErrOverlay": canon.ErrOverlay, "ErrSyntax": canon.ErrSyntax, "ErrClosed": canon.ErrClosed,
	"ErrInternal": canon.ErrInternal,
}

// exitOf is the exit code canon edit and canon rename give an API error.
func exitOf(err error) int {
	inv := &invocation{ctx: context.Background(), env: Env{Stdout: io.Discard, Stderr: io.Discard}, opt: newOptions()}
	inv.opt.format = formatJSON
	return inv.editFail(err, time.Now())
}

// tableRows are the rows of the first Markdown table under a topic's `## <heading>`, cells trimmed.
func tableRows(t *testing.T, topic, heading string) [][]string {
	t.Helper()
	text := guideTexts(t)[topic]
	_, section, ok := strings.Cut(text, "\n## "+heading+"\n")
	if !ok {
		t.Fatalf("%s: no section %q", topic, heading)
	}
	section, _, _ = strings.Cut(section, "\n## ")
	var rows [][]string
	for _, l := range strings.Split(section, "\n") {
		if !strings.HasPrefix(l, "| ") || strings.HasPrefix(l, "|---") {
			continue
		}
		var cells []string
		for _, c := range strings.Split(strings.Trim(l, "|"), " | ") {
			cells = append(cells, strings.TrimSpace(c))
		}
		rows = append(rows, cells)
	}
	if len(rows) < 2 {
		t.Fatalf("%s: section %q has no table", topic, heading)
	}
	return rows[1:]
}

// API.md X1, §15, CLI.md §3.15: each refusal reason is a sentinel's text, with the CLI's exit code for it.
func TestGuideRefusalTable(t *testing.T) {
	for _, row := range tableRows(t, "errors", "Edit and rename refusals") {
		reason := strings.Trim(row[0], "`")
		var match error
		for _, s := range sentinels {
			if reason == s.Error() || strings.HasPrefix(reason, s.Error()+": ") {
				match = s
			}
		}
		if match == nil {
			t.Errorf("errors: %q is the text of no API error", reason)
			continue
		}
		if want := strconv.Itoa(exitOf(match)); row[1] != want {
			t.Errorf("errors: %q exits %s, not %s", reason, want, row[1])
		}
	}
}

// CLI.md §2.5, API.md §15: every API error the exit table names exits with that row's code.
func TestGuideExitTable(t *testing.T) {
	names := regexp.MustCompile("`(Err[A-Za-z]+)`")
	for _, row := range tableRows(t, "cli", "Exit codes") {
		for _, m := range names.FindAllStringSubmatch(row[1], -1) {
			err, ok := sentinels[m[1]]
			if !ok {
				t.Errorf("cli: %s is not an API error", m[1])
				continue
			}
			if got := strconv.Itoa(exitOf(err)); got != row[0] {
				t.Errorf("cli: %s exits %s, the table says %s", m[1], got, row[0])
			}
		}
	}
}

// CLI.md §1: the command table lists exactly the commands of this binary.
func TestGuideCommandTable(t *testing.T) {
	var listed []string
	for _, row := range tableRows(t, "cli", "Commands") {
		words := strings.Fields(strings.Trim(row[0], "`"))
		listed = append(listed, words[1])
	}
	slices.Sort(listed)
	if want := sortedKeys(commands()); !slices.Equal(listed, want) {
		t.Errorf("cli lists %v, the commands are %v", listed, want)
	}
}

// commandLines are the command lines the guide shows: lines of sh fences, split at pipes and
// `&&`, and inline code spans, each kept when it starts with `canon `.
func commandLines(t *testing.T) []string {
	t.Helper()
	var lines []string
	for _, f := range allFences(t) {
		if f.lang != "sh" {
			continue
		}
		for _, l := range strings.Split(f.body, "\n") {
			l, _, _ = strings.Cut(l, " #")
			lines = append(lines, regexp.MustCompile(`\|\|?|&&|;`).Split(l, -1)...)
		}
	}
	for _, text := range guideTexts(t) {
		for _, m := range regexp.MustCompile("`([^`\n]+)`").FindAllStringSubmatch(text, -1) {
			lines = append(lines, m[1])
		}
	}
	var out []string
	for _, l := range lines {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, progName+" ") {
			out = append(out, l)
		}
	}
	return out
}

// CLI.md §1, §2.3: every `canon <command>` shown exists, with only flags it accepts.
func TestGuideCommandsAndFlags(t *testing.T) {
	cmds := commands()
	for _, l := range commandLines(t) {
		words := strings.Fields(l)
		if strings.HasPrefix(words[1], "<") {
			continue // a placeholder: canon <command>
		}
		cmd, ok := cmds[words[1]]
		if !ok {
			t.Errorf("%q: %q is not a command (%v)", l, words[1], sortedKeys(cmds))
			continue
		}
		fs := newFlagSet(newOptions(), cmd.flags)
		for _, w := range words[2:] {
			if name, ok := flagOf(w); ok && fs.Lookup(name) == nil {
				t.Errorf("%q: canon %s has no flag %s", l, words[1], w)
			}
		}
	}
}

// CLI.md §2.3: every flag the guide names in code anywhere is a flag of some command.
func TestGuideFlagsExist(t *testing.T) {
	known := map[string]bool{}
	for _, cmd := range commands() {
		newFlagSet(newOptions(), cmd.flags).VisitAll(func(f *flag.Flag) { known[f.Name] = true })
	}
	known[helpFlag] = true // the flag package's own: usage, exit 2
	texts := guideTexts(t)
	for _, f := range allFences(t) {
		if f.lang == "sh" {
			texts[f.String()] = "`" + strings.ReplaceAll(f.body, "\n", "`\n`") + "`"
		}
	}
	for topic, text := range texts {
		for _, w := range codeWords(text) {
			if name, ok := flagOf(w); ok && !known[name] {
				t.Errorf("%s: no command has the flag %s", topic, w)
			}
		}
	}
}

// codeWords are the words of a text's inline code spans.
func codeWords(text string) []string {
	var out []string
	for _, m := range regexp.MustCompile("`([^`\n]+)`").FindAllStringSubmatch(text, -1) {
		out = append(out, strings.Fields(m[1])...)
	}
	return out
}

// flagOf is the flag a command-line word names: `--name`, `-q`, `--name=value`.
func flagOf(word string) (string, bool) {
	m := regexp.MustCompile(`^--?([a-z][a-z-]*)(=.*)?$`).FindStringSubmatch(word)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// ownKeys are the JSON members of a struct type, untagged embedded structs inlined.
func ownKeys(typs ...reflect.Type) map[string]bool {
	keys := map[string]bool{}
	for _, typ := range typs {
		for i := range typ.NumField() {
			f := typ.Field(i)
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "" && f.Anonymous {
				addAll(keys, ownKeys(f.Type))
			}
			if name != "" && name != "-" {
				keys[name] = true
			}
		}
	}
	return keys
}

func addAll(dst, src map[string]bool) {
	for k := range src {
		dst[k] = true
	}
}

func typeOf(v any) reflect.Type { return reflect.TypeOf(v) }

func fieldType(v any) reflect.Type { return reflect.TypeOf(v).Field(0).Type }

// objectKeys are, by the member holding an object, the members it may have, from their writers.
func objectKeys() map[string]map[string]bool {
	explain, origin := ownKeys(typeOf(explainObject{})), ownKeys(typeOf(originObject{}))
	return map[string]map[string]bool{
		"edit": ownKeys(typeOf(editBody{})), "rename": ownKeys(typeOf(editBody{})),
		"undo": ownKeys(typeOf(editUndo{})), "changes": ownKeys(typeOf(editChange{})),
		"dropped": ownKeys(typeOf(canon.Dropped{})), "explain": explain, "parts": explain,
		"origin": origin, "via": origin, "replaced": origin,
		"ref": ownKeys(fieldType(refLine{})), "test": ownKeys(fieldType(testLine{})),
		"failures": ownKeys(typeOf(failureLine{})), "findings": ownKeys(typeOf(canon.Finding{})),
		"output": ownKeys(fieldType(outputLine{})), "lock": ownKeys(fieldType(lockLine{})),
		"related": ownKeys(typeOf(canon.Related{})), "stack": ownKeys(typeOf(canon.Frame{})),
		"truncated": ownKeys(typeOf(canon.Truncation{})), "version": ownKeys(typeOf(versionJSON{})),
		"summary": ownKeys(fieldType(buildSummary{}), fieldType(testSummary{}), fieldType(refsSummary{}),
			fieldType(fmtSummary{})),
	}
}

// topKeys are the members a top-level JSON object of the guide may have.
func topKeys(obj map[string]any, wrappers map[string]map[string]bool) map[string]bool {
	switch {
	case obj["severity"] != nil:
		return ownKeys(typeOf(canon.Finding{}))
	case obj["ops"] != nil:
		keys := ownKeys(typeOf(canon.Edit{}))
		keys[keyEditLayer] = true
		return keys
	case obj["formatted"] != nil:
		return ownKeys(typeOf(fileLine{}))
	}
	keys := map[string]bool{}
	for k := range wrappers {
		keys[k] = true
	}
	return keys
}

// jsonValues are a fence's JSON values: the whole text, or one per line (JSON Lines).
func jsonValues(f guideFence) ([]any, error) {
	var whole any
	if err := json.Unmarshal([]byte(f.body), &whole); err == nil {
		return []any{whole}, nil
	}
	var out []any
	for _, l := range strings.Split(strings.TrimSpace(f.body), "\n") {
		var v any
		if err := json.Unmarshal([]byte(l), &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// CLI.md §2.4, §3.15, API.md §8.8: each JSON object has only its type's members; requests and ops decode.
func TestGuideJSON(t *testing.T) {
	wrappers := objectKeys()
	for _, f := range allFences(t) {
		if f.lang != "json" || f.arg != "" && !strings.HasPrefix(f.arg, reqDir) {
			continue
		}
		values, err := jsonValues(f)
		if err != nil {
			t.Errorf("%s: %v", f, err)
		}
		for _, v := range values {
			obj, _ := v.(map[string]any)
			checkObject(t, f, "", obj, topKeys(obj, wrappers))
			checkRequest(t, f, obj)
		}
	}
}

// checkRequest decodes a top-level object holding `ops` as canon edit decodes its request.
func checkRequest(t *testing.T, f guideFence, obj map[string]any) {
	t.Helper()
	if obj["ops"] == nil {
		return
	}
	raw, _ := json.Marshal(obj)
	inv := &invocation{opt: newOptions()}
	if _, err := inv.decodeRequest(raw); err != nil {
		t.Errorf("%s: request does not decode: %v", f, err)
	}
}

// enumValues are, by holder and member, the only values a sample may give (API.md §5.2, §8.1; CLI.md §3.5).
var enumValues = map[string][]string{
	"changes.kind": {string(canon.Modified), string(canon.Created), string(canon.Deleted), string(canon.Renamed)},
	"test.status":  {statusPass, statusFail},
	"origin.kind":  originKinds, "via.kind": originKinds, "replaced.kind": originKinds,
}

var originKinds = []string{
	string(canon.OriginLiteral), string(canon.OriginJSON), string(canon.OriginCSV), string(canon.OriginDefines),
	string(canon.OriginText), string(canon.OriginDefault), string(canon.OriginSpread), string(canon.OriginComputed),
	string(canon.OriginLayer),
}

// checkObject checks an object's members against allowed, then each nested object against its own.
func checkObject(t *testing.T, f guideFence, holder string, obj map[string]any, allowed map[string]bool) {
	t.Helper()
	for _, k := range sortedKeys(obj) {
		if values, ok := enumValues[holder+"."+k]; ok && !slices.Contains(values, fmtValue(obj[k])) {
			t.Errorf("%s: %s.%s is %v, not one of %v", f, holder, k, obj[k], values)
		}
		switch {
		case !allowed[k]:
			t.Errorf("%s: %q is not a member here (%v)", f, k, sortedKeys(allowed))
		case k == "value":
		case k == "ops":
			checkOps(t, f, obj[k])
		default:
			checkNested(t, f, k, obj[k])
		}
	}
}

// checkNested checks the objects held by member k, alone or in a list.
func checkNested(t *testing.T, f guideFence, k string, v any) {
	t.Helper()
	wrappers := objectKeys()
	items, isList := v.([]any)
	if !isList {
		items = []any{v}
	}
	for _, item := range items {
		obj, isObject := item.(map[string]any)
		if !isObject {
			continue
		}
		if wrappers[k] == nil {
			t.Errorf("%s: member %q holds no object in any output", f, k)
			continue
		}
		checkObject(t, f, k, obj, wrappers[k])
	}
}

// checkOps decodes each op of an `ops` array as API.md E24 reads it.
func checkOps(t *testing.T, f guideFence, v any) {
	t.Helper()
	ops, _ := v.([]any)
	for _, op := range ops {
		raw, _ := json.Marshal(op)
		var o canon.Op
		if err := o.UnmarshalJSON(raw); err != nil {
			t.Errorf("%s: op %s: %v", f, raw, err)
		}
	}
}

// fmtValue is a JSON scalar as text, so it compares with a constant's.
func fmtValue(v any) string {
	text, _ := v.(string)
	return text
}

// ERRORS.md, DECISIONS 27: every code the guide cites is in the diagnostics registry.
func TestGuideCodesExist(t *testing.T) {
	known := map[string]bool{}
	for _, d := range diag.Registry {
		known[string(d.Code)] = true
	}
	codes := regexp.MustCompile(`\b[EW][0-9]{4}\b`)
	texts := guideTexts(t)
	for _, topic := range sortedKeys(texts) {
		for _, c := range codes.FindAllString(texts[topic], -1) {
			if !known[c] {
				t.Errorf("%s: %s is not a diagnostic code", topic, c)
			}
		}
	}
}
