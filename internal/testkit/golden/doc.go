// Package golden is the golden harness: each case is a txtar archive holding its inputs and,
// in a file named want unless the test names another (Expected), the expected output. Tests
// compare byte for byte; the -update flag rewrites the expected files instead, and the diff is
// reviewed like code.
package golden
