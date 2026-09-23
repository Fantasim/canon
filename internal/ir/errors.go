package ir

import "errors"

// ErrFingerprint is a type the canon-fp v1 grammar cannot print: a malformed TypeRef, or a
// kind with no wire form, which stage E refuses first (E8151).
var ErrFingerprint = errors.New("ir: type has no canon-fp v1 form")
