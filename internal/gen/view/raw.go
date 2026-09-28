package viewgen

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
)

// buildRaw parses raw with jsonsrc.Parse and grafts its tree in as is, its numbers checked for
// WIRE.md 7.2's canonical form (VIEWMODEL.md J10) and never rewritten. A leading UTF-8 BOM
// refuses: a builder-written RawMessage carries none.
func buildRaw(raw json.RawMessage) (*jsonsrc.Node, error) {
	if bytes.HasPrefix(raw, []byte(jsonsrc.UTF8BOM)) {
		return nil, fmt.Errorf("%w: leading UTF-8 BOM", errRaw)
	}
	var fs source.FileSet
	f, err := fs.Add(rawSourceName, rawSourceName, raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRaw, err)
	}
	root, err := jsonsrc.Parse(f, diag.NewBag(&fs, rawSourceName))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRaw, err)
	}
	if err := validateRawNumbers(root); err != nil {
		return nil, err
	}
	return root, nil
}

// validateRawNumbers refuses a non-canonical number anywhere in the tree (WIRE.md 7.2).
func validateRawNumbers(n *jsonsrc.Node) error {
	switch n.Kind {
	case jsonsrc.Null, jsonsrc.Bool, jsonsrc.String:
		return nil
	case jsonsrc.Number:
		return canonicalUnquotedNumber(n.Text)
	case jsonsrc.Array:
		for _, e := range n.Elems {
			if err := validateRawNumbers(e); err != nil {
				return err
			}
		}
	case jsonsrc.Object:
		for _, m := range n.Members {
			if err := validateRawNumbers(m.Value); err != nil {
				return err
			}
		}
	}
	return nil
}
