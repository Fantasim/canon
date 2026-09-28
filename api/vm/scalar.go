package vm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// Number is a number of the view model held as its text: a JSON number, or, for an integer
// outside ±(2^53−1), a decimal string (VIEWMODEL.md J10). The zero Number is absent.
type Number struct {
	Text   string // the number as written, e.g. "100000", "0.5", "9007199254740993"
	Quoted bool   // written as a JSON string (J10)
}

// Scalar is an enum member's wire value or an entry key: a JSON string when Quoted (any text,
// unlike a Number's decimal integer), else an integer's text. Build one with its fields.
type Scalar Number

// Int is the Number of an integer: a JSON number, or a decimal string beyond ±(2^53−1) (J10).
func Int(v int64) Number {
	return Number{Text: strconv.FormatInt(v, 10), Quoted: v > maxSafeInt || v < -maxSafeInt}
}

// MarshalJSON writes the number, refusing a text that is not one (or the absent Number).
func (n Number) MarshalJSON() ([]byte, error) {
	if !n.valid() {
		return nil, fmt.Errorf(fmtBad, ErrNumber, n.Text)
	}
	return writeScalar(n.Text, n.Quoted)
}

// UnmarshalJSON reads a JSON number, or a string holding a decimal integer (J10).
func (n *Number) UnmarshalJSON(b []byte) error {
	text, quoted, err := readScalar(b)
	if err != nil {
		return err
	}
	*n = Number{Text: text, Quoted: quoted}
	if !n.valid() {
		return fmt.Errorf(fmtBad, ErrNumber, text)
	}
	return nil
}

func (n Number) valid() bool {
	if n.Quoted {
		return reDecimal.MatchString(n.Text)
	}
	return reNumber.MatchString(n.Text)
}

// MarshalJSON writes a string, or an integer, refusing an unquoted text that is not one.
func (s Scalar) MarshalJSON() ([]byte, error) {
	if !s.Quoted && !reInteger.MatchString(s.Text) {
		return nil, fmt.Errorf(fmtBad, ErrScalar, s.Text)
	}
	return writeScalar(s.Text, s.Quoted)
}

// UnmarshalJSON reads a JSON string or integer.
func (s *Scalar) UnmarshalJSON(b []byte) error {
	text, quoted, err := readScalar(b)
	if err != nil {
		return err
	}
	if !quoted && !reInteger.MatchString(text) {
		return fmt.Errorf(fmtBad, ErrScalar, text)
	}
	*s = Scalar{Text: text, Quoted: quoted}
	return nil
}

func writeScalar(text string, quoted bool) ([]byte, error) {
	if !quoted {
		return []byte(text), nil
	}
	return marshal(text)
}

// readScalar is the text of a JSON string (unquoted) or of any other JSON value (as written).
func readScalar(b []byte) (string, bool, error) {
	if !bytes.HasPrefix(b, quote) {
		return string(b), false, nil
	}
	var text string
	if err := json.Unmarshal(b, &text); err != nil {
		return "", false, fmt.Errorf(fmtWrap, err)
	}
	return text, true, nil
}

// TextRef is a text reference (VIEWMODEL.md J9): a key "<package>:<key>", or, when Key is
// empty, the language-neutral Text, written {"text": …}. The zero TextRef is absent.
type TextRef struct {
	Key  string
	Text *string
}

type neutralText struct {
	Text *string `json:"text"`
}

// MarshalJSON writes the key as a string or the neutral text as an object; a TextRef with
// both or neither is refused.
func (r TextRef) MarshalJSON() ([]byte, error) {
	if (r.Key == "") == (r.Text == nil) {
		return nil, fmt.Errorf(fmtBad, ErrTextRef, r.Key)
	}
	if r.Text != nil {
		return marshal(neutralText{Text: r.Text})
	}
	return marshal(r.Key)
}

// UnmarshalJSON reads a non-empty key string, or an object holding only "text".
func (r *TextRef) UnmarshalJSON(b []byte) error {
	if bytes.HasPrefix(b, quote) {
		key, _, err := readScalar(b)
		if err != nil {
			return fmt.Errorf(fmtWrapIn, ErrTextRef, err)
		}
		if key == "" {
			return fmt.Errorf(fmtBad, ErrTextRef, b)
		}
		*r = TextRef{Key: key}
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var n neutralText
	if err := dec.Decode(&n); err != nil {
		return fmt.Errorf(fmtWrapIn, ErrTextRef, err)
	}
	if n.Text == nil {
		return fmt.Errorf(fmtBad, ErrTextRef, b)
	}
	*r = TextRef{Text: n.Text}
	return nil
}

func marshal(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	return b, nil
}
