package diag

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// catalogueMessages is the count ERRORS.md states (492 messages).
const catalogueMessages = 492

// sampleFiles holds the files the sample spans point into: sampleExpr covers "a\n\tb".
var sampleFiles = MemFiles{
	{Path: "resource/farm/farm.canon", Content: "let items\na\n\tb = 1\n"},
	{Path: "@resource/Server/System/farm_config.json", Content: "{}\n"},
}

// samples is the sample of each argument type, for the codes that have no constructor.
var samples = map[ArgType]any{
	ArgTypeName: sampleName, ArgTypeNames: sampleNames, ArgTypeChain: sampleChain,
	ArgTypeType: sampleType, ArgTypeTypes: sampleTypes, ArgTypeValue: sampleValue,
	ArgTypeExpr: sampleExpr, ArgTypeInt: sampleInt, ArgTypeRune: sampleRune,
	ArgTypePath: samplePath, ArgTypeLoc: sampleLoc, ArgTypePointer: samplePointer,
	ArgTypeKind: KindEnumMember, ArgTypeText: sampleText, ArgTypeMessage: sampleMessage,
}

// renderedMessage is one message of the registry, rendered from its samples.
type renderedMessage struct {
	def     *Def
	variant int
	args    []any
	text    string
}

// everyMessage renders every message of ERRORS.md: reported codes through their constructors
// (the generated constructed table), runtime codes from the samples of their argument types.
func everyMessage(t *testing.T) []renderedMessage {
	t.Helper()
	var out []renderedMessage
	next := 0
	for i := range Registry {
		d := &Registry[i]
		for v := range d.Variants {
			m := renderedMessage{def: d, variant: v}
			if d.Severity == Runtime {
				m.args, m.text = runtimeMessage(d.Variants[v])
			} else {
				m.args, m.text = constructed[next].args, constructed[next].finding(sampleFiles, sampleName).Message
				next++
			}
			out = append(out, m)
		}
	}
	return out
}

// runtimeMessage renders a runtime code's message from the samples of its argument types.
func runtimeMessage(variant Variant) ([]any, string) {
	var args []any
	for _, a := range variant.Args {
		args = append(args, samples[a.Type])
	}
	return args, renderer{files: sampleFiles}.template(variant.Template, variant.Args, args)
}

// IMPLEMENTATION-PLAN.md §6 M0: every message of ERRORS.md renders from its constructor.
func TestEveryMessageRenders(t *testing.T) {
	r := renderer{files: sampleFiles}
	all := everyMessage(t)
	total := 0
	for _, d := range Registry {
		total += len(d.Variants)
	}
	if len(all) != total || total != catalogueMessages {
		t.Fatalf("rendered %d messages, the registry has %d, ERRORS.md says %d", len(all), total, catalogueMessages)
	}
	for _, m := range all {
		variant := m.def.Variants[m.variant]
		if strings.TrimSpace(m.text) == "" {
			t.Errorf("%s %s: empty message", m.def.Code, variant.Name)
		}
		for i, a := range variant.Args {
			if !strings.Contains(m.text, r.arg(a.Type, m.args[i])) {
				t.Errorf("%s %s: argument %s is not in %q", m.def.Code, variant.Name, a.Name, m.text)
			}
			if strings.Contains(m.text, "{"+a.Name+"}") {
				t.Errorf("%s %s: placeholder {%s} left in %q", m.def.Code, variant.Name, a.Name, m.text)
			}
		}
	}
}

// ERRORS.md §1.2-§1.3: the rendering of every message, reviewed as a golden.
func TestEveryMessageGolden(t *testing.T) {
	golden.Run(t, "testdata/messages.txtar", func(t *testing.T, _ golden.Case) []byte {
		t.Helper()
		var sb strings.Builder
		for _, m := range everyMessage(t) {
			name := m.def.Variants[m.variant].Name
			if name == "" {
				name = "-"
			}
			text := strings.ReplaceAll(m.text, "\n", "\n\t")
			fmt.Fprintf(&sb, "%s %s: %s\n", m.def.Code, name, text)
		}
		return []byte(sb.String())
	})
}

// ERRORS.md §1.2: escapes are read before placeholders; §1.3 renderings of each type.
func TestTemplateAndArguments(t *testing.T) {
	r := renderer{files: sampleFiles}
	params := []Arg{{Name: "name", Type: ArgTypeName}}
	cases := []struct{ got, want string }{
		{r.template(`{{{name}}} \\ a\nb }}`, params, []any{"key"}), "{key} \\ a\nb }"},
		{r.template("{missing}", params, []any{"key"}), ""},
		{r.arg(ArgTypeRune, '\x1f'), "U+001F"},
		{r.arg(ArgTypeRune, rune(0x1F600)), "U+1F600"},
		{r.arg(ArgTypeExpr, sampleExpr), "a b"},
		{r.arg(ArgTypeLoc, sampleLoc), "@resource/Server/System/farm_config.json:1"},
		{r.arg(ArgTypeChain, sampleChain), "a -> b -> a"},
		{r.arg(ArgTypeInt, int64(-42)), "-42"},
		{r.arg(ArgTypePointer, "/modelTypes/3"), "#/modelTypes/3"},
		{r.arg(ArgTypePointer, ""), "#"},
		{r.arg(ArgTypePointer, "/a b/%/é/~1"), "#/a%20b/%25/%C3%A9/~1"},
		{r.arg(ArgTypeType, nil), ""},
		{r.arg(ArgTypeValue, nil), ""},
		{r.arg(ArgTypeMessage, Message{}), ""},
	}
	for i, c := range cases {
		if c.got != c.want {
			t.Errorf("case %d: got %q, want %q", i, c.got, c.want)
		}
	}
}
