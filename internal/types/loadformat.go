package types

import (
	"path"
	"strings"
	"unicode/utf8"
)

// LoadFormat is the format a load reads a file as (WIRE.md §6.2).
type LoadFormat int

// FormatOfPath is a file's format from its name's extension, ASCII case ignored (WIRE.md §6.2).
func FormatOfPath(name string) LoadFormat {
	ext := path.Ext(name)
	for f, w := range formatWords {
		if w.ext != "" && equalFoldASCII(ext, w.ext) {
			return LoadFormat(f)
		}
	}
	return FormatUnknown
}

// FormatNamed is the format a `format:` symbol names, FormatUnknown for any other (WIRE.md §6.2).
func FormatNamed(symbol string) LoadFormat {
	for f, w := range formatWords {
		if w.symbol != "" && w.symbol == symbol {
			return LoadFormat(f)
		}
	}
	return FormatUnknown
}

// FormatFits reports whether a file of format f builds t: text a String, csv per CSVFits, json or unknown anything (WIRE.md §6.7).
func FormatFits(f LoadFormat, header bool, t Type) bool {
	switch f {
	case FormatText:
		return t.Base().Kind() == String
	case FormatCSV:
		return CSVFits(header, t)
	default:
		return true
	}
}

// CSVFits reports `[[String]]` without a header, a list, keyed list or table of records with one (WIRE.md §6.6).
func CSVFits(header bool, t Type) bool {
	if !header {
		return stringRows(t)
	}
	var elem Type
	switch b := t.Base().(type) {
	case *ListType:
		elem = b.Elem
	case *TableType:
		elem = b.Elem
	default:
		return false
	}
	switch elem.Base().(type) {
	case *RecordType, *AppliedRecord:
		return true
	default:
		return false
	}
}

// stringRows reports `[[String]]`: plain lists, not keyed.
func stringRows(t Type) bool {
	rows, ok := t.Base().(*ListType)
	if !ok || rows.KeyedBy != nil {
		return false
	}
	row, ok := rows.Elem.Base().(*ListType)
	return ok && row.KeyedBy == nil && Identical(row.Elem, StringType)
}

// equalFoldASCII reports ext equal to the ASCII want, ASCII case ignored: no other letter folds to one.
func equalFoldASCII(ext, want string) bool {
	for i := range len(ext) {
		if ext[i] >= utf8.RuneSelf {
			return false
		}
	}
	return strings.EqualFold(ext, want)
}
