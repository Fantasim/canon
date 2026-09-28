package jsonschema

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
)

// decodeJSON is strict (internal/jsonsrc.Parse) then builds the value tree (encoding/json,
// json.Number kept: a number compares exactly, never through float64). Every error names label.
func decodeJSON(label string, b []byte) (any, error) {
	var fs source.FileSet
	f, err := fs.Add(label, label, b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	bag := diag.NewBag(&fs, label)
	if _, err := jsonsrc.Parse(f, bag); err != nil {
		return nil, decodeError(label, err, bag)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("%s: decode: %w", label, err)
	}
	return v, nil
}

// decodeError wraps a jsonsrc.Parse failure with label and, when bag holds one, the offset
// and pointer of its first finding: jsonsrc's own diagnostic, otherwise dropped.
func decodeError(label string, err error, bag *diag.Bag) error {
	findings := bag.Findings()
	if len(findings) == 0 {
		return fmt.Errorf("%s: %w", label, err)
	}
	first := findings[0]
	return fmt.Errorf("%s: %w (offset %d, %s: %s)", label, err, first.Span.Start, first.Pointer, first.Message)
}
