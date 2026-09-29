// Package live computes the live view state of one value (spec/API.md 11): the title, subtitle
// and preview of the value, the `when` conditions, `show` lines and resolved dependent types of
// its form, and the heading of every element of the collections the form holds, in a language.
// The caller applies a draft first and fills the result's revision, findings and summary.
package live
