// Package workspace holds a project's snapshots (spec/API.md, rules S1-S11): each call reads one
// immutable snapshot, refreshed from the disk by stat first; a snapshot's revision is the
// SHA-256 of the listing of the files it read, the last revisions are remembered as deltas, and
// a base revision is judged stale per read set. Identical concurrent reads share one
// computation, and a snapshot keeps its analyses; one writer at a time publishes its snapshot
// when it returns, an edit the one its re-check analyzed; overlays replace file contents in
// memory. build stays a stateless pipeline run over a snapshot's file system; subscribers see
// every snapshot published, which Watch builds on.
package workspace
