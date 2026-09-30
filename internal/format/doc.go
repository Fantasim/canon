// Package format prints Canon sources in their one canonical layout (spec/FORMATTER.md): a
// Wadler printer over a document built from the syntax tree, every comment kept next to the
// token it was attached to. Source and File print a whole file, Fresh a file the edit API
// creates. For the edit API's minimal writes, Rewrite applies Replace, Insert, Remove, Retire
// and Move changes to a tree and re-prints only the items they touch; Canonical tells a tree
// already in the layout, judged once per tree; Node prints one node from a column, Flat on one
// line.
package format
