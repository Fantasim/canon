package edit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/fantasim/canonlang/internal/diag"
)

// members is a set of the members of an operation's JSON form.
type members uint8

// opShape is what an operation takes besides `op` and `path`, and what it needs.
type opShape struct {
	takes, needs members
}

// jsonMember is one member of an operation's JSON form: its name, its bit, its reader.
type jsonMember struct {
	name string
	bit  members
	read func(*opDecode, json.RawMessage) error
}

// MarshalJSON writes the compact JSON form of API.md E24-E26; a value without text is ErrNoText.
func (o Operation) MarshalJSON() ([]byte, error) {
	if int(o.Kind) >= len(opNames) {
		return nil, fmt.Errorf(fmtOpJSON, ErrOpJSON, jsonMembers[iOp].name)
	}
	shape := opShapes[o.Kind]
	if err := o.fitsShape(shape); err != nil {
		return nil, err
	}
	b := append([]byte{}, jsonOpen)
	b = appendMember(b, iOp, diag.AppendJSONString(nil, opNames[o.Kind]))
	b = appendMember(b, iPath, diag.AppendJSONString(nil, o.Path))
	if shape.takes&mIndex != 0 {
		b = appendMember(b, iIndex, strconv.AppendInt(nil, int64(o.Index), decimalBase))
	}
	if o.Key != nil {
		text, err := keyJSON(o.Key)
		if err != nil {
			return nil, err
		}
		b = appendMember(b, iKey, text)
	}
	if shape.takes&mCase != 0 {
		b = appendMember(b, iCase, diag.AppendJSONString(nil, o.Case))
	}
	if shape.takes&mName != 0 {
		b = appendMember(b, iName, diag.AppendJSONString(nil, o.Name))
	}
	if o.Value != nil {
		i, text, err := valueJSON(o.Value)
		if err != nil {
			return nil, err
		}
		b = appendMember(b, i, text)
	}
	return append(b, jsonClose), nil
}

// fitsShape refuses a member the operation does not take, or one it needs and lacks.
func (o Operation) fitsShape(shape opShape) error {
	has := members(0)
	for _, m := range []struct {
		bit members
		set bool
	}{{mValue, o.Value != nil}, {mKey, o.Key != nil}, {mCase, o.Case != ""}, {mIndex, o.Index != 0}, {mName, o.Name != ""}} {
		if m.set {
			has |= m.bit
		}
	}
	if extra := has &^ shape.takes; extra != 0 {
		return fmt.Errorf(fmtOpJSON, ErrOpJSON, memberList(extra))
	}
	if missing := shape.needs &^ (has | mIndex | mName); missing != 0 {
		return fmt.Errorf(fmtOpJSON, ErrOpJSON, memberList(missing))
	}
	return nil
}

// appendMember appends `"name":text` for member i, after a comma unless it is the first.
func appendMember(b []byte, i int, text []byte) []byte {
	if len(b) > 1 {
		b = append(b, jsonComma)
	}
	b = diag.AppendJSONString(b, jsonMembers[i].name)
	b = append(b, jsonColon)
	return append(b, text...)
}

// keyJSON is a key as E25 writes it: a JSON string, or an integer for an integer key.
func keyJSON(k Lit) ([]byte, error) {
	switch x := k.(type) {
	case PathKey:
		return diag.AppendJSONString(nil, string(x)), nil
	case Key:
		return diag.AppendJSONString(nil, string(x)), nil
	case Member:
		return diag.AppendJSONString(nil, string(x)), nil
	case Str:
		return diag.AppendJSONString(nil, string(x)), nil
	case IntKey:
		return strconv.AppendInt(nil, int64(x), decimalBase), nil
	case Int:
		return strconv.AppendInt(nil, int64(x), decimalBase), nil
	}
	return nil, fmt.Errorf(fmtOpJSON, ErrOpJSON, jsonMembers[iKey].name)
}

// valueJSON is a value's member and text (E26): a FromJSON compacted as `value`, any other
// value as `source`, its Canon literal laid out by format.Flat.
func valueJSON(v Lit) (int, []byte, error) {
	if x, ok := v.(FromJSON); ok {
		var b bytes.Buffer
		if err := json.Compact(&b, x); err != nil {
			return 0, nil, fmt.Errorf(fmtOpJSONErr, ErrOpJSON, jsonMembers[iValue].name, err)
		}
		return iValue, b.Bytes(), nil
	}
	text, err := sourceText(v)
	if err != nil {
		return 0, nil, fmt.Errorf(fmtOpJSON, ErrNoText, describe(v))
	}
	return iSource, diag.AppendJSONString(nil, text), nil
}

// UnmarshalJSON reads the JSON form of API.md E24-E26, refusing unknown, repeated and misplaced members.
func (o *Operation) UnmarshalJSON(data []byte) error {
	ms, err := readObject(data)
	if err != nil {
		return err
	}
	var d opDecode
	for _, m := range ms {
		jm, ok := memberNamed(m.name)
		if !ok || d.seen&jm.bit != 0 {
			return fmt.Errorf(fmtOpJSON, ErrOpJSON, m.name)
		}
		d.seen |= jm.bit
		if err := jm.read(&d, m.raw); err != nil {
			return fmt.Errorf(fmtOpJSONErr, ErrOpJSON, m.name, err)
		}
	}
	if err := d.complete(); err != nil {
		return err
	}
	*o = d.op
	return nil
}

// memberNamed is the member of that name; `value` and `source` share a bit, so both is twice (E24).
func memberNamed(name string) (jsonMember, bool) {
	for _, m := range jsonMembers {
		if m.name == name {
			return m, true
		}
	}
	return jsonMember{}, false
}

// rawMember is one member of an object as read, in order.
type rawMember struct {
	name string
	raw  json.RawMessage
}

// readObject is the members of the JSON object data, in order, duplicates kept.
func readObject(data []byte) ([]rawMember, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil || tok != json.Delim(jsonOpen) {
		return nil, fmt.Errorf(fmtOpJSON, ErrOpJSON, textObject)
	}
	var out []rawMember
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf(fmtOpJSONErr, ErrOpJSON, textObject, err)
		}
		var m rawMember
		m.name, _ = tok.(string)
		if err := dec.Decode(&m.raw); err != nil {
			return nil, fmt.Errorf(fmtOpJSONErr, ErrOpJSON, m.name, err)
		}
		out = append(out, m)
	}
	if tok, err := dec.Token(); err != nil || tok != json.Delim(jsonClose) {
		return nil, fmt.Errorf(fmtOpJSON, ErrOpJSON, textObject)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf(fmtOpJSON, ErrOpJSON, textObject)
	}
	return out, nil
}

// opDecode is an operation being read, and the members read so far.
type opDecode struct {
	op   Operation
	seen members
}

// complete checks the members read against the operation's shape.
func (d *opDecode) complete() error {
	if d.seen&(mOp|mPath) != mOp|mPath {
		return fmt.Errorf(fmtOpJSON, ErrOpJSON, memberList((mOp|mPath)&^d.seen))
	}
	shape := opShapes[d.op.Kind]
	if extra := d.seen &^ (shape.takes | mOp | mPath); extra != 0 {
		return fmt.Errorf(fmtOpJSON, ErrOpJSON, memberList(extra))
	}
	if missing := shape.needs &^ d.seen; missing != 0 {
		return fmt.Errorf(fmtOpJSON, ErrOpJSON, memberList(missing))
	}
	return nil
}

func readOp(d *opDecode, raw json.RawMessage) error {
	name, err := jsonString(raw)
	if err != nil {
		return err
	}
	for k := OpSet; k <= OpRenameName; k++ {
		if opNames[k] == name {
			d.op.Kind = k
			return nil
		}
	}
	return errUnknownOp
}

func readPath(d *opDecode, raw json.RawMessage) (err error) {
	d.op.Path, err = jsonString(raw)
	return err
}

// readCase is SetCase's case, a name: never empty.
func readCase(d *opDecode, raw json.RawMessage) (err error) {
	d.op.Case, err = jsonString(raw)
	if err == nil && d.op.Case == "" {
		return errEmptyCase
	}
	return err
}

// readName is RenameName's new name, an empty one included: E30 judges it (E24).
func readName(d *opDecode, raw json.RawMessage) (err error) {
	d.op.Name, err = jsonString(raw)
	return err
}

// readValue is a wire value (FromJSON); `null` is None (E24).
func readValue(d *opDecode, raw json.RawMessage) error {
	if len(raw) > 0 && raw[0] == jsonNullByte {
		d.op.Value = None{}
		return nil
	}
	d.op.Value = FromJSON(bytes.Clone(raw))
	return nil
}

func readSource(d *opDecode, raw json.RawMessage) error {
	text, err := jsonString(raw)
	d.op.Value = Source(text)
	return err
}

// readKey is a JSON string, read later as a path key, or an integer (E25).
func readKey(d *opDecode, raw json.RawMessage) error {
	if len(raw) > 0 && raw[0] == jsonQuote[0] {
		text, err := jsonString(raw)
		d.op.Key = PathKey(text)
		return err
	}
	n, err := strconv.ParseInt(string(raw), decimalBase, int64Bits)
	if err != nil {
		return fmt.Errorf(fmtBadMember, errNotInteger, err)
	}
	d.op.Key = IntKey(n)
	return nil
}

func readIndex(d *opDecode, raw json.RawMessage) error {
	n, err := strconv.Atoi(string(raw))
	if err != nil {
		return fmt.Errorf(fmtBadMember, errNotInteger, err)
	}
	d.op.Index = n
	return nil
}

// jsonString is the JSON string raw holds.
func jsonString(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf(fmtBadMember, errNotString, err)
	}
	return s, nil
}

// memberList is the names of the members of set, in writing order.
func memberList(set members) string {
	var names []byte
	for _, m := range jsonMembers {
		if set&m.bit == 0 {
			continue
		}
		set &^= m.bit
		if len(names) > 0 {
			names = append(names, listSep...)
		}
		names = append(names, m.name...)
	}
	return string(names)
}
