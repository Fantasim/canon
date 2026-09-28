package edit_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// IMPLEMENTATION-PLAN.md §7.7 path round trip through Resolve: canonical paths resolve to themselves.
func FuzzResolve(f *testing.F) {
	for _, seed := range []string{
		"a:byColor[red]", `a:byColor["GREEN"]`, `a:named["two words"]`, "a:numbered[-4]", "a:items.b", "a:items[#1].n",
		"a:rows[7].label", "a:tones[green]", "a:links[one].to", "a:shape.kind", "a:shape.r", "a:derived.sub.n",
		"a:entries.two.id", "a:entries[#1].retired", "a:perEntry[two]", "a:holder.data.n", "a:notes", "a:KS[#0]",
		"a:Color.green", "Color.red", "shared", "b:shared", "a:hidden", "a:wrap.code", "a:mixed[0]", "a:viaName.n",
		"a:base.sub", "a:bag.xs[#2]", "a:bag.ts[red].w", "a:bag.byC[green]", "a:bag.b.sub.n", "a:bag.xs[2]", "u:tagged[red]", `u:tagged["none"]`, "u:tagged[#1]", "a:defs.A.value", "a:other.data.n", "a:byColor.red", "a:shape[0]",
	} {
		f.Add(seed)
	}
	fx := lawFixture(f, []string{"dev"}, "a", "b", "u")
	f.Fuzz(func(t *testing.T, in string) {
		p, err := edit.Parse(in)
		if err != nil {
			return
		}
		r, err := edit.Resolve(fx.Snapshot, p)
		var pe *edit.PathError
		if err != nil {
			if !errors.As(err, &pe) {
				t.Fatalf("Resolve(%q): %v is not a *PathError", in, err)
			}
			return
		}
		if strings.Contains(r.Canonical, "[#") {
			t.Fatalf("Resolve(%q).Canonical = %s holds a position", in, r.Canonical)
		}
		cp, err := edit.Parse(r.Canonical)
		if err != nil {
			t.Fatalf("canonical %s of %q: %v", r.Canonical, in, err)
		}
		again, err := edit.Resolve(fx.Snapshot, cp)
		if err != nil || again.Canonical != r.Canonical || text(again) != text(r) {
			t.Fatalf("canonical %s of %q resolves to %s, %v", r.Canonical, in, again.Canonical, err)
		}
		for op := edit.OpSet; op <= edit.OpSetCase; op++ {
			for _, layer := range []string{"", "dev", "stage", "lc", "ld", "le", "lf", "la"} {
				if _, err := fx.Editable(r, op, layer); err != nil {
					t.Fatalf("Editable(%s, %d, %q): %v", r.Canonical, op, layer, err)
				}
			}
		}
	})
}

func text(r edit.Resolved) string {
	if r.Target == nil {
		return ""
	}
	return r.Target.CanonText()
}
