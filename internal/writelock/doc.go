// Package writelock is the advisory writer lock per package of DECISIONS 352: the file
// .canon/writers/pkg.<package>, held through flock on Unix and CreateFile share modes on Windows,
// released by the system when its process ends, never deleted, holding one line "host pid".
//
// Acquire takes requests in package-name order, all or none, each polled until free or the
// context is done; WithWaiting's function is told once whom a writer waits for. A build takes
// its selected packages Exclusive and its other loaded ones Shared; fmt and an edit take each
// package they write Exclusive. An edit runs API.md N9 under the locks and asks Held.Holds whether
// a new plan's packages are held; if not, it releases, acquires the full sorted set, plans again.
package writelock
