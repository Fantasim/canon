package cppgen_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// docSpliceWant is what the driver prints once every getter and enumerator is declared.
const docSpliceWant = "label=x count=3 dir=src/ kind=c\nmembers=8 a=a b=b d=d e=e\n"

// CODEGEN.md §2.6, CG-06, DECISIONS 423: a doc with a line ending in a backslash is written as a block comment, so C++ line splicing does not pull the next generated line into it; the header compiles under -Wall -Wextra -Werror and keeps every getter and enumerator.
func TestDocBackslashCompiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, docSplicePackage()), "docsplice_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"docsplice.gen.cpp", "main.cpp"}) {
		if out != docSpliceWant {
			t.Errorf("driver output:\n%s\nwant:\n%s", out, docSpliceWant)
		}
	}
}

// CODEGEN.md §2.6, CG-06, DECISIONS 423: in no generated file does a line end in a backslash outside a block comment, and a block comment ends only at its own ` */`.
func TestDocNoSpliceOutsideBlock(t *testing.T) {
	t.Parallel()
	for _, f := range generate(t, docSplicePackage()) {
		inBlock := false
		for n, l := range strings.Split(string(f.Content), "\n") {
			trimmed := strings.TrimSpace(l)
			switch {
			case inBlock && trimmed == "*/":
				inBlock = false
			case !inBlock && trimmed == "/**":
				inBlock = true
			case inBlock && (strings.Contains(l, "*/") || strings.Contains(l, "/*")):
				t.Errorf("%s:%d: a block comment's text holds a comment delimiter: %q", f.Path, n+1, l)
			case !inBlock && strings.HasSuffix(strings.TrimRight(l, " \t"), `\`):
				t.Errorf("%s:%d: a line outside a block comment ends in a backslash: %q", f.Path, n+1, l)
			}
		}
		if inBlock {
			t.Errorf("%s: a block comment never closes", f.Path)
		}
		if strings.Contains(string(f.Content), trigraphSlash) {
			t.Errorf("%s: holds the trigraph %s", f.Path, trigraphSlash)
		}
	}
}

// trigraphSlash is the trigraph for a backslash, which g++ -Wtrigraphs refuses and -std=c++14 splices on.
const trigraphSlash = "??/"

// CODEGEN.md §2.6, §2.7, DECISIONS 423: the text of a block-form doc picks no standard header, as a `///` doc's never does.
func TestDocBlockAddsNoInclude(t *testing.T) {
	t.Parallel()
	for _, f := range generate(t, docSplicePackage()) {
		if f.Path != "docsplice.gen.h" {
			continue
		}
		for _, h := range []string{"<vector>", "<chrono>"} {
			if strings.Contains(string(f.Content), "#include "+h) {
				t.Errorf("%s includes %s because a doc names it", f.Path, h)
			}
		}
	}
}

// CODEGEN.md §2.6, CG-06, DECISIONS 423: the block form is ` * ` + line (` *` for an empty line) with `*/` written `*\/`; a doc without a line ending in a backslash keeps `///`, `*/` and a backslash inside a line included.
func TestDocForms(t *testing.T) {
	t.Parallel()
	var header string
	for _, f := range generate(t, docSplicePackage()) {
		if filepath.Ext(f.Path) == ".h" && strings.HasPrefix(f.Path, "docsplice") {
			header = string(f.Content)
		}
	}
	tests := []struct{ name, want string }{
		{"enum", "/**\n * A kind of splice \\\n */\nenum class Kind"},
		{"member", "    /**\n     * The first member \\\n     */\n    a,\n"},
		{"plain member", "    /// The second member, plain.\n    b,\n"},
		{"escape, empty line", "    /**\n     * A comment end *\\/ here\n     * and a line ending in \\\n     *\n     * after an empty line\n     */\n    c,\n"},
		{"escape /* and overlaps", "    /**\n     * Reads src/\\*.json \\\n     * then /\\*\\/ and *\\/\\* and *\\/\\*\\/ too\n     * and a comment /\\* start\n     */\n    e,\n"},
		{"blanks after the backslash", "    /**\n     * Blanks follow the backslash \\ \t\n     * and std::vector and std::chrono::seconds are only words\n     */\n    f,\n"},
		{"trigraph in a plain doc", "    /// A trigraph ends this line ?\\?/\n    /// and ?\\?/ sits mid line, and a run ends ??\\?/\n    g,\n"},
		{"trigraph in a block doc", "    /**\n     * A block with a trigraph ?\\?/\n     * and a backslash \\\n     */\n    h,\n"},
		{"slash then backslash", "    /**\n     * The directory, ending in a slash: src/\\\n     */\n    const std::string& GetDir() const { return dir_; }\n"},
		{"plain keeps verbatim", "    /// Plain, with a comment end */ and a path C:\\dir\\x\n    d,\n"},
		{"record", "/**\n * A record whose doc ends in a backslash \\\n * and goes on.\n */\nclass Item {"},
		{"field", "    /**\n     * The label, ending in a backslash \\\n     */\n    const std::string& GetLabel() const { return label_; }\n"},
		{"plain field", "    /// The count, plain.\n    int64_t GetCount() const { return count_; }\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(header, tt.want) {
				t.Errorf("header lacks:\n%s\nin:\n%s", tt.want, header)
			}
		})
	}
}
