package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

// node is one JSON value of the schema. An object keeps its members in document order, which
// the generated field order follows (VIEWMODEL.md J2).
type node struct {
	kind nodeKind
	keys []string // object member names
	vals []*node  // object member values, parallel to keys, or array elements
	text string   // string value, number text, or "true", "false", "null"
}

func parseSchema(data []byte) (*node, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	n, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errTrailing
	}
	return n, nil
}

func parseValue(dec *json.Decoder) (*node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf(fmtSchema, err)
	}
	switch t := tok.(type) {
	case json.Delim:
		if t == '{' {
			return parseObject(dec)
		}
		return parseArray(dec)
	case string:
		return &node{kind: kindString, text: t}, nil
	case json.Number:
		return &node{kind: kindNumber, text: t.String()}, nil
	case bool:
		return &node{kind: kindBool, text: strconv.FormatBool(t)}, nil
	}
	return &node{kind: kindNull, text: "null"}, nil
}

func parseObject(dec *json.Decoder) (*node, error) {
	n := &node{kind: kindObject}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf(fmtSchema, err)
		}
		key, _ := tok.(string)
		if slices.Contains(n.keys, key) {
			return nil, fmt.Errorf(fmtAt, errDuplicate, key)
		}
		v, err := parseValue(dec)
		if err != nil {
			return nil, err
		}
		n.keys, n.vals = append(n.keys, key), append(n.vals, v)
	}
	return n, closeDelim(dec)
}

func parseArray(dec *json.Decoder) (*node, error) {
	n := &node{kind: kindArray}
	for dec.More() {
		v, err := parseValue(dec)
		if err != nil {
			return nil, err
		}
		n.vals = append(n.vals, v)
	}
	return n, closeDelim(dec)
}

func closeDelim(dec *json.Decoder) error {
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf(fmtSchema, err)
	}
	return nil
}

// get is the member key of an object node; nil when n is not an object or has no such member.
func (n *node) get(key string) *node {
	if n == nil || n.kind != kindObject {
		return nil
	}
	if i := slices.Index(n.keys, key); i >= 0 {
		return n.vals[i]
	}
	return nil
}

func (n *node) has(key string) bool { return n.get(key) != nil }

// texts lists the string elements of an array node (nil for a missing node).
func (n *node) texts() []string {
	if n == nil {
		return nil
	}
	out := make([]string, 0, len(n.vals))
	for _, v := range n.vals {
		out = append(out, v.text)
	}
	return out
}

// canonical is n's compact text, member order kept: two equal texts are the same schema.
func (n *node) canonical() string {
	var b strings.Builder
	n.write(&b)
	return b.String()
}

func (n *node) write(b *strings.Builder) {
	switch n.kind {
	case kindObject:
		b.WriteString("{")
		for i, k := range n.keys {
			b.WriteString(strconv.Quote(k) + ":")
			n.vals[i].write(b)
			b.WriteString(",")
		}
		b.WriteString("}")
	case kindArray:
		b.WriteString("[")
		for _, v := range n.vals {
			v.write(b)
			b.WriteString(",")
		}
		b.WriteString("]")
	case kindString:
		b.WriteString(strconv.Quote(n.text))
	default:
		b.WriteString(n.text)
	}
}

// textOr is the text of a node, "" for a missing one.
func (n *node) textOr() string {
	if n == nil {
		return ""
	}
	return n.text
}

// keysOr is the member names of an object node (nil for a missing node).
func (n *node) keysOr() []string {
	if n == nil {
		return nil
	}
	return n.keys
}

// valsOr is the elements of an array node, or the member values of an object node.
func (n *node) valsOr() []*node {
	if n == nil {
		return nil
	}
	return n.vals
}
