// Package build runs a project's build: parse, check, evaluate, verify, the checks and the
// precomputations, then the emit of code, data and canon.lock, never over a file canon did not
// generate: all or nothing on a write error; a crash between renames can leave a mix.
//
// A Project given a Cache (WithCache), one per workspace project, keeps the parses of unchanged
// files, re-checks an edit of entry values alone with check.Session.Recheck and replays the
// entries it did not change from eval's memo (NFR-02); each result equals a cold one.
package build
