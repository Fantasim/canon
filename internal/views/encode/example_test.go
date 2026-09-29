package encode_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/encode"
)

// Uncatalogued labels are neutral (VIEWMODEL.md J9); big integers are strings (J10).
func Example() {
	label := encode.NewTexts(nil).Label("a", "Title", "title", "Quest", i18n.FieldSeg("title"))
	fmt.Println(i18n.FieldSeg("title"), *label.Text)
	fmt.Println(string(encode.Value(&value.List{Elems: []value.Value{
		&value.Int{V: 1 << 60, T: types.IntType}, &value.Str{V: "x", T: types.StringType},
	}})))
	// Output:
	// field.title title
	// ["1152921504606846976","x"]
}
