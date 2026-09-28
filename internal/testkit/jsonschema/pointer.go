package jsonschema

import (
	"fmt"
	"strconv"
	"strings"
)

// Violation is one failed assertion, at the instance pointer Instance and the schema pointer
// Keyword (a $ref jump rewrites Keyword to point at the def, not the $ref site).
type Violation struct {
	Instance string
	Keyword  string
	Message  string
}

func (e Violation) String() string {
	return fmt.Sprintf("%s: %s: %s", e.Instance, e.Keyword, e.Message)
}

// appendToken escapes one pointer token and appends it to base.
func appendToken(base, token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	token = strings.ReplaceAll(token, "/", "~1")
	return base + "/" + token
}

// appendIndex appends an array index token to base.
func appendIndex(base string, i int) string {
	return base + "/" + strconv.Itoa(i)
}
