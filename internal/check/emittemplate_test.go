package check_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// templateProject gives root gen a template for five targets: {dir}, {dotted}, both, none (GRAMMAR.md §7.1).
const templateProject = `project acme {
  canon: "0.1"
  roots {
    gen: "gen"
  }
  out_templates {
    go: { gen: "internal/{dir}" }
    ts: { gen: "web/{dotted}.ts" }
    cpp: { gen: "src/{dir}/{dotted}" }
    json: { gen: "data" }
    text: { gen: "t/{dir}" }
  }
}
`

// loadProject loads text as project.canon, with the findings it gives: nil after an error one.
func loadProject(t *testing.T, text string) (*project.Project, []diag.Finding) {
	t.Helper()
	set := &source.FileSet{}
	src, err := set.Add(projectFile, "/"+projectFile, []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	bag := diag.NewBag(set, "")
	p, err := project.Load(src, bag)
	if err != nil && (!errors.Is(err, project.ErrInvalid) || bag.ErrorCount() == 0) {
		t.Fatal(err)
	}
	return p, bag.Findings()
}

// outValue is the value of the out option of `emit <target> { out: <out> }`, parsed.
func outValue(t *testing.T, target, out string) syntax.Expr {
	t.Helper()
	set := &source.FileSet{}
	src, err := set.Add("a/a.canon", "/a/a.canon", []byte("package a\n\nemit "+target+" { out: "+out+" }\n"))
	if err != nil {
		t.Fatal(err)
	}
	f := syntax.Parse(src, syntax.FileSource, diag.NewBag(set, "a"))
	return f.Decls[0].(*syntax.EmitDecl).Options.Items[0].(*syntax.FieldItem).Value
}

// CODEGEN.md §2.1 "Templates" and "Copies", GRAMMAR.md §5.3, DECISIONS 229, 439.
func TestOutTemplateExpansion(t *testing.T) {
	p, found := loadProject(t, templateProject)
	if len(found) != 0 {
		t.Fatalf("project findings: %v", found)
	}
	for _, c := range []struct {
		name, target, pkg, out string
		want                   []string
	}{
		{"{dir}", check.TargetGo, "items.vocab", "gen", []string{"@gen/internal/items/vocab"}},
		{"{dotted}", check.TargetTS, "items.vocab", "gen", []string{"@gen/web/items.vocab.ts"}},
		{"both", check.TargetCpp, "items.vocab", "gen", []string{"@gen/src/items/vocab/items.vocab"}},
		{"none", check.TargetJSON, "items.vocab", "gen", []string{"@gen/data"}},
		{"a single-segment package", check.TargetText, "a", "gen", []string{"@gen/t/a"}},
		{"a list mixing names and paths", check.TargetGo, "a", `[gen, "@gen/x", other, 3]`, []string{"@gen/internal/a", "@gen/x", "", ""}},
		{"a quoted root name with a template", check.TargetGo, "a", `"gen"`, []string{""}},
		{"a quoted root name without one", check.TargetCpp, "a", `["./gen", "gen/"]`, []string{"./gen", "gen/"}},
		{"view takes one entry", check.TargetView, "a", `[gen]`, []string{""}},
		{"an interpolation", check.TargetGo, "a", `"x{y}"`, []string{""}},
		{"a raw string", check.TargetGo, "a", `r"@gen/r"`, []string{"@gen/r"}},
	} {
		x := outValue(t, c.target, c.out)
		got := check.OutTexts(p, c.pkg, c.target, x)
		texts := make([]string, len(got))
		for i, en := range got {
			texts[i] = en.Text
		}
		if !slices.Equal(texts, c.want) {
			t.Errorf("%s: OutTexts(%s) = %q, want %q", c.name, c.out, texts, c.want)
		}
		if !slices.Equal(entryAts(got), entryNodes(x, c.target)) {
			t.Errorf("%s: an entry is not at its node", c.name)
		}
	}
	if got := check.OutTexts(nil, "a", check.TargetGo, outValue(t, check.TargetGo, "gen")); len(got) != 1 || got[0].Text != "" {
		t.Errorf("without a project a root name names no template: %v", got)
	}
}

// entryAts are the nodes of entries.
func entryAts(entries []check.OutText) []syntax.Expr {
	out := make([]syntax.Expr, len(entries))
	for i, en := range entries {
		out[i] = en.At
	}
	return out
}

// entryNodes are the nodes of out's entries: a list's elements but for view, else out itself.
func entryNodes(out syntax.Expr, target string) []syntax.Expr {
	if list, isList := out.(*syntax.ListLit); isList && target != check.TargetView {
		return list.Elems
	}
	return []syntax.Expr{out}
}

// templateFor is a project whose root gen has the template written as the string literal lit for target.
func templateFor(target, lit string) string {
	return "project acme {\n  canon: \"0.1\"\n  roots {\n    gen: \"gen\"\n  }\n  out_templates {\n    " + target + ": { gen: " + lit + " }\n  }\n}\n"
}

// GRAMMAR.md §7.1, CODEGEN.md §2.1, DECISIONS 439: out_templates takes exactly check's emit targets, only ts needing .ts.
func TestTemplateTargetsMatchEmitTargets(t *testing.T) {
	targets := check.EmitTargets()
	if len(targets) == 0 {
		t.Fatal("no emit target")
	}
	for _, target := range targets {
		if _, found := loadProject(t, templateFor(target, `"x.ts"`)); len(found) != 0 {
			t.Errorf("target %s refused in out_templates: %v", target, found)
		}
		_, found := loadProject(t, templateFor(target, `"x"`))
		if refused := len(found) != 0; refused != (target == check.TargetTS) {
			t.Errorf("target %s, a template not ending in %s: findings %v", target, check.TSSuffix, found)
		}
	}
	for _, word := range []string{"rust", "Go", "views", "texts", "jsonl", "c"} {
		_, found := loadProject(t, templateFor(word, `"x"`))
		if len(found) != 1 || found[0].Code != diag.E1019.Def().Code {
			t.Errorf("%s is no emit target, yet out_templates gives %v", word, found)
		}
	}
}

// GRAMMAR.md §7.1, CODEGEN.md §2.1, DECISIONS 424, 439: a template uses exactly the characters an out may resolve to.
func TestTemplateCharsetMatchesOut(t *testing.T) {
	runes := []rune("é€ ")
	for r := rune(' '); r <= '~'; r++ {
		runes = append(runes, r)
	}
	for _, r := range runes {
		lit := `r"a` + string(r) + `b"`
		if r == '"' {
			lit = `"a\"b"`
		}
		_, found := loadProject(t, templateFor(check.TargetGo, lit))
		if accepted, portable := len(found) == 0, strings.ContainsRune(check.OutCharset, r); accepted != portable {
			t.Errorf("%q: template accepted %v, out charset holds it %v (%v)", r, accepted, portable, found)
		}
	}
}
