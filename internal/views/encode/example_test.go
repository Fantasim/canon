package encode_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/encode"
)

// A reserved field name goes behind its kind word (I18N.md K4), a text without a letter is
// language-neutral (L7), and an integer past 2^53−1 is a decimal string (VIEWMODEL.md J10).
func Example() {
	fmt.Println(encode.Key("a", "Quest", encode.FieldSeg("title")).Key, *encode.Plain("—", "a", "Quest", "help").Text)
	fmt.Println(string(encode.Value(&value.List{Elems: []value.Value{
		&value.Int{V: 1 << 60, T: types.IntType}, &value.Str{V: "x", T: types.StringType},
	}})))
	// Output:
	// a:Quest.field.title —
	// ["1152921504606846976","x"]
}
