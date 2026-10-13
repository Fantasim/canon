package golden

import (
	"path/filepath"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

const (
	duplicateKeyProject = "testdata/membership/duplicatekey.txtar"

	// duplicateKeyWant is DECISIONS 361's texts and CODEGEN.md §5.13's order: the first failure in document order, each named after the type; a byte is 0-based, in the bytes as given.
	duplicateKeyWant = "Holder: $.d: duplicate key\n" +
		"Holder: invalid JSON at byte 6\nHolder: invalid JSON at byte 6\nHolder: invalid JSON at byte 11\nHolder: invalid JSON at byte 8\n" +
		"Holder: invalid JSON at byte 0\nHolder: invalid JSON at byte 3\nHolder: invalid JSON at byte 5\n" +
		"Holder: $.a: duplicate key\nHolder: $.a: duplicate key\n" +
		"Holder: $.a: not UTF-8\nHolder: $.a: unpaired surrogate\nHolder: $.a: unpaired surrogate\n" +
		"Holder: invalid JSON at byte 8\nHolder: invalid JSON at byte 14\nHolder: invalid JSON at byte 9\n" +
		"Holder: $.ab: duplicate key\nHolder: $.x.ab: duplicate key\n" +
		"Holder: $: not UTF-8\nHolder: $: unpaired surrogate\nHolder: $: not UTF-8\nHolder: $.x: unpaired surrogate\n" +
		"Holder: $.a: unpaired surrogate\nHolder: $.a: unpaired surrogate\nHolder: $.a: not UTF-8\nHolder: invalid JSON at byte 11\n" +
		"Holder: $.f: number out of range\nHolder: $.f: number out of range\n<nil>\nHolder: $: expected an object\n" +
		"depth ok\ndepth ok\ndepth ok\ndepth ok\n" +
		"HoldersFile: invalid JSON at byte 1\nHoldersFile: $[0].d: duplicate key\n"

	duplicateKeyRunner = `package main

import (
	"fmt"
	"strings"

	demo "example.com/gen/go"
)

func main() {
	for _, text := range []string{"{\"d\": \"hp\", \"d\": \"mp\"}", "{\"d\": ", "{\"d\": x}", "{\"d\":\"hp\"} x", "\xef\xbb\xbf{\"d\" 1}",
		"", "\xef\xbb\xbf", "{\"a\":\xff}", "{\"a\":1,\"a\":2,", "{\"a\":1,\"a\":2} x",
		"{\"a\": \"\xff", "{\"a\": \"\\udc00", "{\"a\": \"\\ud800 x", "{\"a\": \"\xc3", "{\"a\": \"\\ud800\\", "{\"a\":1,\"a",
		"{\"a\\u0062\":1,\"ab\":2}", "{\"x\":{\"a\":1,\"a\\u0062\":2,\"\\u0061b\":3}}",
		"{\"a\xff\":1}", "{\"\\ud800\":1}", "{\"a\":1,\"\xff\":2}", "{\"x\":{\"\\udc00\":1}}",
		"{\"a\": \"\\udc00\xff\"}", "{\"a\": \"\\ud800x\xff\"}", "{\"a\": \"\xff\\udc00\"}", "{\"a\": \"\\udc",
		"{\"d\":\"hp\",\"f\":1e400}", "{\"d\":\"hp\",\"f\":-1e400}", "{\"d\":\"hp\",\"zz\":1e400}", "1e400"} {
		_, err := demo.DecodeHolder([]byte(text))
		fmt.Println(err)
	}
	want := "Holder: $" + strings.Repeat("[0]", 512) + ": nested deeper than 512 levels"
	for _, text := range []string{strings.Repeat("[", 600), strings.Repeat("[", 600) + strings.Repeat("]", 600),
		strings.Repeat("[", 10002) + strings.Repeat("]", 10002), strings.Repeat("[", 20000)} {
		if _, err := demo.DecodeHolder([]byte(text)); err == nil || err.Error() != want {
			fmt.Println("depth:", err)
			continue
		}
		fmt.Println("depth ok")
	}
	for _, text := range []string{"[,", "[{\"d\": \"hp\", \"d\": \"hp\"}]"} {
		_, err := demo.DecodeHoldersFile([]byte(text))
		fmt.Println(err)
	}
}
`
)

// DECISIONS 361 and its Review, CODEGEN.md §5.13, WIRE.md §3.1-§3.2: a Go types-mode decoder reports the first failure in document order, a failure's place being the byte at which it becomes certain (a string a syntax error leaves unclosed is checked for UTF-8 and surrogates, not for a repeated key): `duplicate key` at its path, nesting past 512 at the 513th level (whatever depth encoding/json allows), and a syntax error as `invalid JSON at byte N`, a repeated key before it winning.
func TestTypesDecoderSaysDuplicateKey(t *testing.T) {
	dir, p := openProject(t, duplicateKeyProject)
	gen := buildProject(t, p, dir, canon.TargetGo)
	writeTestFile(t, filepath.Join(gen, goModFile), []byte(goModText))
	writeTestFile(t, filepath.Join(gen, runDir, goMain), []byte(duplicateKeyRunner))
	if got := run(t, gen, "go", "run", "./"+runDir); got != duplicateKeyWant {
		t.Errorf("go prints %q, want %q", got, duplicateKeyWant)
	}
}
