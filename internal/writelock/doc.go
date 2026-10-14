// Package writelock is the advisory writer lock per package of DECISIONS 352: the file
// .canon/writers/pkg.<package>, held through flock on Unix and CreateFile share modes on Windows,
// released by the system when its process ends, never deleted, holding one line "host pid".
//
// Acquire takes requests in package-name order, all or none, each polled until free or the
// context is done; WithWaiting's function is told once whom a writer waits for. A build takes
// its selected packages Exclusive and its other loaded ones Shared; fmt and an edit take each
// package they write Exclusive. TryAcquire is Acquire that never waits: all or none, ok false
// when a lock is held (by this process too: flock is per open file), ErrNoLocks where there is
// no primitive. A journal's writer holds its locks, so a held lock is its liveness (DECISIONS 375).
package writelock
