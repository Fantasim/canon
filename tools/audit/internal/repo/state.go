package repo

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

// StateRow is one line of .sovaudit/state.tsv as written, valid or not.
type StateRow struct {
	Line int
	Rule string
	Mode string
}

// ReadStateRows parses state.tsv text: `<rule>\t<mode>`, # comments and blanks ignored.
func ReadStateRows(text string) []StateRow {
	var out []StateRow
	sc := bufio.NewScanner(strings.NewReader(text))
	for n := 1; sc.Scan(); n++ {
		l := strings.TrimSpace(sc.Text())
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		f := strings.Fields(l)
		row := StateRow{Line: n, Rule: f[0]}
		if len(f) > 1 {
			row.Mode = f[1]
		}
		out = append(out, row)
	}
	return out
}

// loadStates reads the valid rows of a state file; a missing file is no overrides.
func loadStates(path string) (map[string]rules.Mode, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]rules.Mode{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	out := map[string]rules.Mode{}
	for _, row := range ReadStateRows(string(data)) {
		if _, ok := rules.Lookup(row.Rule); ok && rules.ValidMode(rules.Mode(row.Mode)) {
			out[row.Rule] = rules.Mode(row.Mode)
		}
	}
	return out, nil
}

// WriteStates writes overrides sorted by rule id.
func WriteStates(path string, states map[string]rules.Mode) error {
	var b strings.Builder
	b.WriteString(stateHeader)
	for _, id := range rules.IDs() {
		if m, ok := states[id]; ok {
			fmt.Fprintf(&b, "%s\t%s\n", id, m)
		}
	}
	if err := os.WriteFile(path, []byte(b.String()), filePerm); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
