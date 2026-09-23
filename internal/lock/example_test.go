package lock_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/lock"
)

// A lock is read as a set of facts and always written back in canonical order (LOCK.md §2).
func ExampleParse() {
	data := []byte("# canon.lock v1\ntable  teamboard.statuses  open\nenum  teamboard.Tone  2  warm\n")
	files := oneFile{path: "teamboard/canon.lock", content: data}
	f, ok := lock.Parse(1, data, "teamboard", diag.NewBag(files, "teamboard"))
	changed, err := f.Add(lock.Fact{Kind: lock.KindTable, Name: "teamboard.statuses", Holder: "open", Retired: true})
	fmt.Printf("%v %v %v %d %q\n", ok, changed, err, len(f.Facts()), f.Format())
	_, err = f.Add(lock.Fact{Kind: lock.KindEnum, Name: "teamboard.Tone", Value: lock.Value{IsString: true, Str: "3"}, Holder: "cold"})
	fmt.Println(err)
	// Output: true true <nil> 2 "# canon.lock v1\nenum   teamboard.Tone  2  warm\ntable  teamboard.statuses  open  retired\n"
	// invalid lock fact: an enum code is an integer: "enum   teamboard.Tone  \"3\"  cold"
}
