// Package canon is the public Go API of the Canon compiler: the library behind the canon
// command, the language server and the studio; the frozen contract spec/API.md describes.
//
// Open rolls back an unfinished edit first. A Project holds snapshots, each call reading one,
// refreshed from the disk or kept fresh by Watch; overlays replace files in memory. Reads
// (Check, Value, Refs, ViewModel, Evaluate, Test, LockCheck, Build with Check) run in parallel
// and share identical computations; Edit, a writing Build and the overlays are the one writer.
// Edit applies operations in memory, refuses them in API.md E21's order, then writes minimally
// and atomically through a journal and publishes one event; Evaluate applies a draft the same
// way, never writing. A compiler panic is an *InternalError; every error wraps a sentinel.
package canon
