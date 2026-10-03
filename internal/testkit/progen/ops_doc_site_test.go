package progen_test

import (
	"bytes"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

const (
	farmSource      = "resource/farm/farm.canon"
	farmFrench      = "resource/farm/farm.fr.canon"
	farmLevelsHelp  = "ModelType.levels.help "
	farmLevelsField = "levels"
)

// TestUndocumentedFieldSkipsTranslatedHelp: a field whose help a translation keys keeps its doc (I18N.md F4).
func TestUndocumentedFieldSkipsTranslatedHelp(t *testing.T) {
	c := examples(t)
	if n := levelsSites(t, c.targets); n != 0 {
		t.Errorf("%s: %d sites removing the doc of %s, whose help %s translates; want 0", farmSource, n, farmLevelsField, farmFrench)
	}
	q := c.project.Clone()
	fr, _ := q.Get(farmFrench)
	at := bytes.Index(fr, []byte(farmLevelsHelp))
	if at < 0 {
		t.Fatalf("%s no longer translates %s", farmFrench, farmLevelsHelp)
	}
	end := at + bytes.IndexByte(fr[at:], '\n') + 1
	q.Set(farmFrench, slices.Concat(fr[:at], fr[end:]))
	if n := levelsSites(t, targetsOf(q, []string{"resource.farm"})); n != 1 {
		t.Errorf("%s without that translation: %d sites on %s, want 1", farmSource, n, farmLevelsField)
	}
}

// levelsSites counts the W1002 operator's sites that remove the doc of the farm's levels field.
func levelsSites(t *testing.T, targets []target) int {
	t.Helper()
	i := slices.IndexFunc(targets, func(tg target) bool { return tg.path == farmSource })
	if i < 0 {
		t.Fatalf("no target %s", farmSource)
	}
	return len(slices.DeleteFunc(undocumentedField(targets[i]), func(s progen.Site) bool {
		return !slices.ContainsFunc(s.Edits, func(e progen.Edit) bool { return e.Text == farmLevelsField })
	}))
}
