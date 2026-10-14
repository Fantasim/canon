package edit

import (
	"reflect"
	"testing"
)

// API.md E17, DECISIONS 375 (J7): a plan's packages are the owners of the files it writes, a rename's
// old place included; a written file of no package is Unowned; a removed directory is no file.
func TestOwnedUnowned(t *testing.T) {
	a := &applier{owners: map[string]string{"a/x.canon": "a", "b/old.canon": "b"}}
	changes := []Change{
		{Kind: ChangeModified, Path: "a/x.canon"},
		{Kind: ChangeRenamed, OldPath: "b/old.canon", Path: "c/new.canon"},
		{Kind: ChangeCreated, Path: "@data/loose.json"},
		{Kind: ChangeRemovedDir, Path: "b/sub"},
	}
	touched, unowned := a.owned(changes)
	if !reflect.DeepEqual(touched, []string{"a", "b"}) || !reflect.DeepEqual(unowned, []string{"@data/loose.json", "c/new.canon"}) {
		t.Fatalf("touched %v, unowned %v", touched, unowned)
	}
	if touched, unowned = a.owned(changes[:1]); unowned != nil || len(touched) != 1 {
		t.Fatalf("touched %v, unowned %v", touched, unowned)
	}
}
