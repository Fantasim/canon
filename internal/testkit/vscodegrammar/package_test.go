package vscodegrammar

import (
	"encoding/json"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/project"
)

type manifest struct {
	Main    string `json:"main"`
	Contrib struct {
		Languages []struct {
			ID         string   `json:"id"`
			Extensions []string `json:"extensions"`
		} `json:"languages"`
		Grammars []struct {
			Language string `json:"language"`
			Scope    string `json:"scopeName"`
			Path     string `json:"path"`
		} `json:"grammars"`
	} `json:"contributes"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// CLI.md 4: the extension registers the canon language and its grammar, starts canon lsp and
// watches the files a project loads; with no `transport` (it would add --stdio, which canon lsp refuses), and it declares exactly the
// approved packages (IMPLEMENTATION-PLAN.md 11).
func TestManifest(t *testing.T) {
	data, err := os.ReadFile(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Contrib.Languages) != 1 || m.Contrib.Languages[0].ID != languageID ||
		!slices.Equal(m.Contrib.Languages[0].Extensions, []string{project.SourceExt}) {
		t.Errorf("languages: %+v", m.Contrib.Languages)
	}
	if len(m.Contrib.Grammars) != 1 || m.Contrib.Grammars[0].Language != languageID ||
		m.Contrib.Grammars[0].Path != manifestGrammar {
		t.Errorf("grammars: %+v", m.Contrib.Grammars)
	}
	if !maps.Equal(m.Dependencies, map[string]string{languageClient: languageClientRange}) ||
		!maps.Equal(m.DevDependencies, map[string]string{vsce: vsceRange}) {
		t.Errorf("dependencies: %v %v", m.Dependencies, m.DevDependencies)
	}
	ext, err := os.ReadFile(extensionPath)
	if err != nil {
		t.Fatal(err)
	}
	code := codeOnly(string(ext))
	for _, want := range []string{watcherGlob, serverArgs, "context.subscriptions.push"} {
		if !strings.Contains(code, want) {
			t.Errorf("extension.js lacks %s", want)
		}
	}
	if strings.Contains(code, "transport") {
		t.Error("extension.js sets a transport: the client would append --stdio")
	}
}

// codeOnly drops the // comment lines of a JavaScript file.
func codeOnly(src string) string {
	var b strings.Builder
	for _, l := range strings.Split(src, lineSeparator) {
		if !strings.HasPrefix(strings.TrimSpace(l), "//") {
			b.WriteString(l + lineSeparator)
		}
	}
	return b.String()
}
