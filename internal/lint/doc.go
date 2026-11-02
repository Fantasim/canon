// Package lint is the advisory lints of `canon lint` (DECISIONS 440): warnings W0001 to W0008
// about legacy smells that are valid Canon, read from the analysis `canon check` makes, and the
// edit requests that fix the ones a request can fix.
//
// Run fills one diag.Bag per selected package, so findings come in F2 order and are truncated as
// every read is (API.md F7). A broken declaration is never judged; a lint writes no file.
package lint
