// Package build runs a project's build: parse, check, evaluate, verify, the checks and the
// precomputations, then the emit of code, data and canon.lock, never over a file canon did not
// generate: all or nothing on a write error; a crash between renames can leave a mix.
package build
