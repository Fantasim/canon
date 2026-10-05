package wire

import (
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/value"
)

// Text is the §5 encoding of v as a `@text` file, pretty at depth 0 and one LF, WIRE.md §8.5, DECISIONS 308.
func Text(v value.Value) ([]byte, error) {
	n, err := (&encoder{}).value(v, scope{})
	if err != nil {
		return nil, err
	}
	return jsonsrc.Format(n.toJSON()), nil
}
