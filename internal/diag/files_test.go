package diag

import (
	"bytes"

	"github.com/fantasim/canonlang/internal/source"
)

// MemFile is one file of a MemFiles: its display path and content.
type MemFile struct {
	Path    string
	Content string
}

// MemFiles is an in-memory Files for tests: file id i+1 is element i, id 0 has no file.
type MemFiles []MemFile

func (m MemFiles) file(id source.FileID) (MemFile, bool) {
	if id == 0 || int(id) > len(m) {
		return MemFile{}, false
	}
	return m[id-1], true
}

// Path is the display path of the file, "" for none.
func (m MemFiles) Path(id source.FileID) string {
	f, _ := m.file(id)
	return f.Path
}

// Content is the file's bytes.
func (m MemFiles) Content(id source.FileID) []byte {
	f, _ := m.file(id)
	return []byte(f.Content)
}

// Position is the 1-based line and byte column of p, clamped to the content.
func (m MemFiles) Position(id source.FileID, p source.Pos) (line, col int) {
	f, _ := m.file(id)
	before := []byte(f.Content[:min(max(int(p), 0), len(f.Content))])
	start := bytes.LastIndexByte(before, '\n') + 1
	return bytes.Count(before, []byte{'\n'}) + 1, len(before) - start + 1
}
