package winpaths_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/testkit/winpaths"
)

// Example shows a UNC share's volume as Windows returns it, backslashes and all.
func Example() {
	fmt.Println(winpaths.VolumeName("//server/share/proj/a.canon"))
	// Output: \\server\share
}
