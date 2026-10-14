package edit_test

import (
	"bytes"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/writelock"
)

// otherRev and otherJournal are a second commit's revision and journal, beside testRev's.
var (
	otherRev     = "r1:" + strings.Repeat("cd", 32)
	otherJournal = projectDir + "/.canon/journal/" + strings.Repeat("cd", 32) + ".json"
)

// otherChange creates a file no other commit of these tests touches.
var otherChange = []edit.Change{{Kind: edit.ChangeCreated, Path: "zz.canon", After: []byte("zz\n")}}

// API.md N11 (DECISIONS 375: J6): a commit holding b applies beside a version-2 journal on a; holding a,
// the journal meets it and refuses it; a commit taking no lock is refused by any journal.
func TestCommitIgnoresDisjointJournal(t *testing.T) {
	cases := []struct {
		name string
		site func(d *diskFS) edit.Site
		ok   bool
	}{
		{"disjoint", func(d *diskFS) edit.Site { return lockSite(d, newLocker(), "b") }, true},
		{"meeting", func(d *diskFS) edit.Site { return lockSite(d, newLocker(), "a", "b") }, false},
		{"no lock", func(d *diskFS) edit.Site { return siteOf(d) }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, _ := crashedHolding(t, "a")
			before := d.state()
			err := edit.Commit(t.Context(), tc.site(d), otherRev, otherChange)
			if tc.ok {
				if err != nil || string(d.files["/p/zz.canon"]) != "zz\n" || d.files[journalFile] == nil {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if !errors.Is(err, edit.ErrJournal) || !strings.Contains(err.Error(), strings.Repeat("ab", 32)) {
				t.Fatalf("err = %v", err)
			}
			mustState(t, d.state(), before)
		})
	}
}

// API.md N11 (DECISIONS 375: J6): a commit holding b is still refused by another host's journal, one
// recording no lock, one that does not decode or has an unknown version, and one at its own name.
func TestCommitRefusesForeignLocklessOwnName(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T) *diskFS
	}{
		{"foreign", func(t *testing.T) *diskFS {
			d, _ := crashedHolding(t, "a")
			rewrite(t, d, journalFile, setKey("host", "host-b"))
			return d
		}},
		{"lockless", func(t *testing.T) *diskFS { d, _ := crashed(t); return d }},
		{"torn", func(t *testing.T) *diskFS {
			d, _ := crashedHolding(t, "a")
			d.files[journalFile] = []byte("{")
			return d
		}},
		{"unknown version", func(t *testing.T) *diskFS {
			d, _ := crashedHolding(t, "a")
			rewrite(t, d, journalFile, setKey("version", 9))
			return d
		}},
		{"bad lock list", func(t *testing.T) *diskFS {
			d, _ := crashedHolding(t, "a")
			rewrite(t, d, journalFile, setKey("locks", []any{}))
			return d
		}},
		{"own name", func(t *testing.T) *diskFS {
			d, _ := crashedHolding(t, "a")
			d.files[otherJournal] = d.files[journalFile]
			delete(d.files, journalFile)
			return d
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := tc.setup(t)
			before := d.state()
			if err := edit.Commit(t.Context(), lockSite(d, newLocker(), "b"), otherRev, otherChange); !errors.Is(err, edit.ErrJournal) {
				t.Fatalf("err = %v", err)
			}
			mustState(t, d.state(), before)
		})
	}
}

// API.md N10 (DECISIONS 375: J2): every commit writes version 2; its lock list, right after the pid, is
// what the site holds, sorted and unique, absent where it takes no lock; a Locker holding none refuses.
func TestCommitJournalLocks(t *testing.T) {
	cases := []struct {
		name string
		site func(d *faultFS) edit.Site
		want string
	}{
		{"held", func(d *faultFS) edit.Site { return lockSite(d, newLocker(), "pkg", "a", "pkg") }, `"pid":7,"locks":["a","pkg"],"files"`},
		{"no locker", func(d *faultFS) edit.Site { return siteOf(d) }, `"pid":7,"files"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, changes := commitFixture()
			var data []byte
			f := &faultFS{diskFS: d, onOp: func(op, n string, b []byte) {
				if n == journalFile && op == opWrite {
					data = b
				}
			}}
			if err := edit.Commit(t.Context(), tc.site(f), testRev, changes); err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(data, []byte(`{"version":2,`)) || !bytes.Contains(data, []byte(tc.want)) {
				t.Fatalf("journal %s", data)
			}
		})
	}
	refused := []struct {
		held []string
		want error
	}{{[]string{"a/b"}, writelock.ErrBadPackage}, {nil, edit.ErrNoHeld}, {[]string{}, edit.ErrNoHeld}}
	for _, tc := range refused {
		d, changes := commitFixture()
		before := d.state()
		if err := edit.Commit(t.Context(), lockSite(d, newLocker(), tc.held...), testRev, changes); !errors.Is(err, tc.want) {
			t.Fatalf("held %q: err = %v, want %v", tc.held, err, tc.want)
		}
		mustState(t, d.state(), before)
	}
}

// API.md N11 (DECISIONS 375: J6): a journal removed between the listing and its reading (another
// recovery finished it) does not block a commit, nor fail Recover.
func TestJournalVanishes(t *testing.T) {
	for _, recover := range []bool{false, true} {
		d, before := crashedHolding(t, "a")
		f := &faultFS{diskFS: d}
		f.onRead = func(name string) {
			if name == journalFile {
				_ = d.Remove(name)
			}
		}
		if recover {
			if err := edit.Recover(lockSite(f, newLocker()), nil); err != nil {
				t.Fatalf("Recover: %v", err)
			}
			if d.state() == before {
				t.Fatal("rolled back a journal it never read")
			}
			continue
		}
		if err := edit.Commit(t.Context(), lockSite(f, newLocker(), "a"), otherRev, otherChange); err != nil {
			t.Fatalf("Commit: %v", err)
		}
	}
}

// API.md N11 (DECISIONS 375: J4, J11): Settle rolls back a journal all of whose locks the site holds,
// returns the files it restored or removed, logs one Warn line; one not covered, another host's
// or one recording no lock stays, and a site with no Locker settles nothing.
func TestSettleRecoversMeeting(t *testing.T) {
	d, before := crashedHolding(t, "a", "b")
	crash := d.state()
	for _, s := range []edit.Site{siteOf(d), lockSite(d, newLocker(), "a")} {
		if changed, err := edit.Settle(s, nil); err != nil || changed != nil {
			t.Fatalf("changed %v, err %v", changed, err)
		}
		mustState(t, d.state(), crash)
	}
	var buf bytes.Buffer
	changed, err := edit.Settle(lockSite(d, newLocker(), "a", "b", "c"), slog.New(slog.NewTextHandler(&buf, nil)))
	if err != nil || !reflect.DeepEqual(changed, []string{"a.canon", "b.canon"}) {
		t.Fatalf("changed %v, err %v", changed, err)
	}
	mustState(t, d.state(), before)
	if lines := strings.Count(buf.String(), "level=WARN"); lines != 1 || !strings.Contains(buf.String(), "journals=1") {
		t.Fatalf("logged %q", buf.String())
	}
	for _, mutate := range []func(map[string]any){setKey("host", "host-b"), setKey("locks", nil)} {
		d, _ := crashedHolding(t, "a")
		rewrite(t, d, journalFile, mutate)
		crash := d.state()
		if changed, err := edit.Settle(lockSite(d, newLocker(), "a"), nil); err != nil || changed != nil {
			t.Fatalf("changed %v, err %v", changed, err)
		}
		mustState(t, d.state(), crash)
	}
}

// API.md N11 (DECISIONS 375: J4): Settle that cannot roll a covered journal back keeps it and says so.
func TestSettleKeepsConflict(t *testing.T) {
	d, _ := crashedHolding(t, "a")
	d.files["/p/a.canon"] = []byte("someone else\n")
	if _, err := edit.Settle(lockSite(d, newLocker(), "a"), nil); !errors.Is(err, edit.ErrJournal) || d.files[journalFile] == nil {
		t.Fatalf("err = %v", err)
	}
}

// DECISIONS 375 (J4): Meeting is the transitive closure of the journals meeting what is held: {a,b} and
// {b,c} from a give a, b, c; {x,y}, another host's {a,q} and a lockless journal never count.
func TestMeetingClosure(t *testing.T) {
	d := newDiskFS(map[string]string{"/p/project.canon": "project\n"})
	d.mkdirs("/p/.canon/journal")
	plantJournal := func(hex, host string, locks ...string) {
		j := `{"version":2,"revision":"r1:` + hex + `","host":"` + host + `","pid":1`
		if len(locks) > 0 {
			j += `,"locks":["` + strings.Join(locks, `","`) + `"]`
		}
		d.files["/p/.canon/journal/"+hex+".json"] = []byte(j + `,"files":[]}`)
	}
	plantJournal(strings.Repeat("1", 64), testHost, "a", "b")
	plantJournal(strings.Repeat("2", 64), testHost, "b", "c")
	plantJournal(strings.Repeat("3", 64), testHost, "x", "y")
	plantJournal(strings.Repeat("4", 64), "host-b", "a", "q")
	plantJournal(strings.Repeat("5", 64), testHost)
	cases := []struct {
		holding, want []string
	}{
		{[]string{"a"}, []string{"a", "b", "c"}},
		{[]string{"c"}, []string{"a", "b", "c"}},
		{[]string{"y", "z"}, []string{"x", "y"}},
		{[]string{"z"}, nil},
	}
	for _, tc := range cases {
		need, err := edit.Meeting(lockSite(d, newLocker()), tc.holding)
		if err != nil || !reflect.DeepEqual(need, tc.want) {
			t.Errorf("Meeting(%v) = %v, %v; want %v", tc.holding, need, err, tc.want)
		}
	}
	if need, err := edit.Meeting(siteOf(d), []string{"a"}); need != nil || err != nil {
		t.Errorf("no Locker: %v, %v", need, err)
	}
}
