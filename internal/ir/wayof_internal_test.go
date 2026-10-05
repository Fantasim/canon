package ir

import (
	"os"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

const (
	errorsDoc  = "../../spec/ERRORS.md"
	kindsTitle = "### 1.4 Kinds"
	wayPrefix  = "Way"
	tableCell  = "|"
	backtick   = "`"
)

// DECISIONS 305, ERRORS.md §1.4: every kind ERRORS.md lists for E8019, but a way out itself, has its way out in wayOf, the Way<Kind> row of the same name.
func TestWayOfCoversEveryE8019Kind(t *testing.T) {
	data, err := os.ReadFile(errorsDoc)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]diag.Kind{}
	for k := diag.Kind(0); k.String() != ""; k++ {
		byName[k.String()] = k
	}
	code := string(diag.E8019.Def().Code)
	n := 0
	for _, name := range e8019Kinds(string(data), code) {
		n++
		k, ok := byName[name]
		if !ok {
			t.Errorf("ERRORS.md kind %s is not generated", name)
			continue
		}
		if way, ok := wayOf[k]; !ok || way.String() != wayPrefix+name {
			t.Errorf("wayOf[%s] = %v, want %s", name, way, wayPrefix+name)
		}
	}
	if n == 0 {
		t.Fatalf("no %s kind found in %s", code, errorsDoc)
	}
}

// e8019Kinds are the names of §1.4's rows whose Used by column lists code, the Way rows left out.
func e8019Kinds(doc, code string) []string {
	_, rest, _ := strings.Cut(doc, kindsTitle)
	var out []string
	for _, line := range strings.Split(rest, "\n") {
		if strings.HasPrefix(line, "### ") {
			break
		}
		cells := strings.Split(line, tableCell)
		if len(cells) < 4 {
			continue
		}
		name := strings.Trim(strings.TrimSpace(cells[1]), backtick)
		if strings.Contains(cells[3], code) && !strings.HasPrefix(name, wayPrefix) {
			out = append(out, name)
		}
	}
	return out
}
