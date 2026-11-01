package specfence

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// Find is the body of the first block of lang after heading in text, with the line break that ends its last line.
func Find(text, heading, lang string) (string, error) {
	_, rest, ok := strings.Cut(text, heading)
	if !ok {
		return "", fmt.Errorf("%q: %w", heading, ErrNoHeading)
	}
	_, body, ok := strings.Cut(rest, fenceMark+lang+newline)
	if !ok {
		return "", fmt.Errorf("%s after %q: %w", lang, heading, ErrNoBlock)
	}
	block, _, ok := strings.Cut(body, fenceClose)
	if !ok {
		return "", fmt.Errorf("%s after %q: %w", lang, heading, ErrUnclosed)
	}
	return block + newline, nil
}

// Block reads the document at path and fails tb at once when it cannot be read or holds no such block.
func Block(tb testing.TB, path, heading, lang string) string {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatal(err)
	}
	block, err := Find(string(data), heading, lang)
	if err != nil {
		tb.Fatalf("%s: %v", path, err)
	}
	return block
}
