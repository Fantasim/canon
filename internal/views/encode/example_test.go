package encode_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/encode"
)

// Reserved names take their kind word (I18N.md K4), uncatalogued labels are neutral (J9), big
// integers are strings (J10).
func Example() {
	label := encode.NewTexts(nil).Label("a", "Title", "title", "Quest", encode.FieldSeg("title"))
	fmt.Println(encode.FieldSeg("title"), *label.Text)
	fmt.Println(string(encode.Value(&value.List{Elems: []value.Value{
		&value.Int{V: 1 << 60, T: types.IntType}, &value.Str{V: "x", T: types.StringType},
	}})))
	// Output:
	// field.title title
	// ["1152921504606846976","x"]
}
