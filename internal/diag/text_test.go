package diag_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// lineAt is the span of the first byte of a 1-based line of file id in files, plus width bytes.
func lineAt(files diag.MemFiles, id source.FileID, line, col, width int) source.Span {
	content := files[id-1].Content
	off := 0
	for range line - 1 {
		off += strings.IndexByte(content[off:], '\n') + 1
	}
	start := source.Pos(off + col - 1)
	return source.Span{File: id, Start: start, End: start + source.Pos(width)}
}

// numbered is n lines of text, each naming its number.
func numbered(n int) string {
	var sb strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "line %d of the file\n", i)
	}
	return sb.String()
}

// render renders findings with a bag's summary into a string.
func render(t *testing.T, files diag.Files, bag *diag.Bag, opt diag.RenderOptions) []byte {
	t.Helper()
	opt.Summary = opt.Summary.Merge(bag.Summary())
	var buf bytes.Buffer
	if err := diag.Render(&buf, files, bag.Findings(), opt); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// API.md F9, F10, F11, F12, F13, F14, F15: the heistia and farm examples' findings.txt, byte for byte.
func TestTextFormEqualsExampleFindings(t *testing.T) {
	heistia := diag.MemFiles{
		{Path: "@resource/Server/System/heistia_config.json", Content: "{}\n"},
		{Path: "resource/heistia/heistia.canon", Content: numbered(70)},
	}
	farm := diag.MemFiles{
		{Path: "@resource/Server/System/farm_config.json", Content: numbered(20)},
		{Path: "resource/farm/farm.canon", Content: numbered(120)},
		{Path: "resource/farm/farm.fr.canon", Content: numbered(20)},
	}
	cases := []struct {
		golden string
		files  diag.MemFiles
		report func(files diag.MemFiles, bag *diag.Bag)
	}{
		{"../../examples/resource/heistia/expected/findings.txt", heistia, func(files diag.MemFiles, bag *diag.Bag) {
			diag.W1701.AtMany(lineAt(files, 2, 9, 1, 1), 44, "resource.heistia", "fr").Report(bag)
			diag.W5001.At(lineAt(files, 1, 1, 1, 2), "two tasks have the same description").Path("heistia").
				Related(lineAt(files, 2, 62, 3, 5), diag.NoteCheck("")).Report(bag)
		}},
		{"../../examples/resource/farm/expected/findings.txt", farm, func(files diag.MemFiles, bag *diag.Bag) {
			diag.W5001.At(lineAt(files, 1, 17, 19, 1), "maxLevel is 3 but only 2 levels exist: level 3 and above are unreachable (GetLevel returns nullptr)").
				Path("farm.modelTypes[1].maxLevel").Check("unreachable_levels").
				Related(lineAt(files, 2, 109, 3, 4), diag.NoteCheck("unreachable_levels")).Report(bag)
			diag.W1701.AtMany(lineAt(files, 3, 14, 1, 1), 39, "resource.farm", "fr").Report(bag)
		}},
	}
	for _, c := range cases {
		want, err := os.ReadFile(c.golden)
		if err != nil {
			t.Fatal(err)
		}
		bag := diag.NewBag(c.files, "resource.test")
		c.report(c.files, bag)
		if got := render(t, c.files, bag, diag.RenderOptions{Golden: true}); !bytes.Equal(got, want) {
			t.Errorf("%s:\n--- got\n%s--- want\n%s", c.golden, got, want)
		}
	}
}

// scenarios build the findings of each golden case of testdata/render from its files.
var scenarios = map[string]func(t *testing.T, files diag.MemFiles) (*diag.Bag, diag.RenderOptions){
	"text.txtar": func(t *testing.T, files diag.MemFiles) (*diag.Bag, diag.RenderOptions) {
		t.Helper()
		return mixed(files), diag.RenderOptions{Duration: 1400 * time.Millisecond}
	},
	"json.txtar": func(t *testing.T, files diag.MemFiles) (*diag.Bag, diag.RenderOptions) {
		t.Helper()
		return mixed(files), diag.RenderOptions{Format: diag.FormatJSON, Duration: 1400 * time.Millisecond}
	},
	"truncated.txtar": func(t *testing.T, files diag.MemFiles) (*diag.Bag, diag.RenderOptions) {
		t.Helper()
		return truncated(files, diag.FormatText)
	},
	"truncated_json.txtar": func(t *testing.T, files diag.MemFiles) (*diag.Bag, diag.RenderOptions) {
		t.Helper()
		return truncated(files, diag.FormatJSON)
	},
	"clean.txtar": func(t *testing.T, files diag.MemFiles) (*diag.Bag, diag.RenderOptions) {
		t.Helper()
		return diag.NewBag(files, "teamboard"), diag.RenderOptions{Golden: true}
	},
}

// mixed reports one finding of each shape: multi-line, stack cut, path, pointer, layer,
// source note, no file, check with reads, and a duplicate.
func mixed(files diag.MemFiles) *diag.Bag {
	bag := diag.NewBag(files, "teamboard")
	diag.E6001.AtRenamed(lineAt(files, 1, 33, 1, 19), "teamboard.statuses.wont_do", "rejected", "wont_do").Report(bag)
	frames := make([]diag.Frame, 18)
	for i := range frames {
		frames[i] = diag.Frame{Fn: fmt.Sprintf("teamboard.step%d", i), Span: lineAt(files, 2, i+1, 3, 4)}
	}
	diag.E4001.At(lineAt(files, 2, 40, 6, 2), lineAt(files, 2, 40, 6, 2)).Stack(frames).MoreFrames(3).Report(bag)
	decl := lineAt(files, 2, 41, 1, 19)
	diag.E3501.At(lineAt(files, 3, 12, 6, 7), valueText(`"II_SYS_SYS_SCR_FARM3"`), "resource.vocab.items").
		Path("farm.modelTypes[3].levels[2].productionItem").Pointer("/modelTypes/3/levels/2/productionItemDefine").
		Layer("gm_test").Related(decl, diag.NoteSource(decl)).Report(bag)
	diag.E1003.At(source.Span{}, "/home/someone/project").Report(bag)
	diag.E1003.At(source.Span{}, "/home/someone/project").Report(bag)
	diag.W5001.At(lineAt(files, 3, 2, 1, 1), "a <b> & \"c\"\tU+2028:\u2028").Check("unreachable_levels").
		Path("farm").Reads([]string{"maxLevel", "levels"}).
		Related(lineAt(files, 2, 44, 3, 1), diag.NoteCheck("unreachable_levels")).Report(bag)
	return bag
}

// truncated keeps two findings of five, and merges the summary of a second package.
func truncated(files diag.MemFiles, format diag.Format) (*diag.Bag, diag.RenderOptions) {
	bag := diag.NewBag(files, "teamboard")
	bag.Truncate(2)
	for line := 5; line >= 1; line-- {
		diag.E1004.At(lineAt(files, 1, line, 1, 1)).Report(bag)
	}
	other := diag.NewBag(files, "resource.vocab")
	diag.W6006.AtOne(lineAt(files, 1, 1, 1, 1)).Report(other)
	return bag, diag.RenderOptions{Format: format, Summary: other.Summary(), Duration: 12 * time.Millisecond}
}

// API.md F5, F9, F10, F11, F12, F13, F14, F15: the text and JSON forms of testdata/render, as goldens.
func TestRenderGoldens(t *testing.T) {
	golden.Run(t, "testdata/render/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		var files diag.MemFiles
		for _, f := range c.Archive.Files {
			if f.Name != "want" {
				files = append(files, diag.MemFile{Path: f.Name, Content: string(f.Data)})
			}
		}
		scenario, ok := scenarios[filepath.Base(c.Path)]
		if !ok {
			t.Fatalf("no scenario for %s", c.Path)
		}
		bag, opt := scenario(t, files)
		return render(t, files, bag, opt)
	})
}
