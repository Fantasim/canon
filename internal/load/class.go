package load

import "strings"

// rewriteForDoublestar transforms seg's classes from WIRE.md's own reading into doublestar's.
func rewriteForDoublestar(seg string) string {
	var b strings.Builder
	for i := 0; i < len(seg); i++ {
		if seg[i] != '[' {
			b.WriteByte(seg[i])
			continue
		}
		end, ok := skipClass(seg, i)
		if !ok {
			b.WriteByte(seg[i])
			continue
		}
		b.WriteString(rewriteClass(seg[i:end]))
		i = end - 1
	}
	return b.String()
}

// rewriteClass is one closed "[...]" class, its leading "]" or "^" escaped to a literal.
func rewriteClass(cls string) string {
	body, prefix := cls[1:len(cls)-1], "["
	if strings.HasPrefix(body, "!") {
		prefix, body = prefix+"!", body[1:]
	}
	switch {
	case strings.HasPrefix(body, "]"):
		body = `\]` + body[1:]
	case strings.HasPrefix(body, "^"):
		body = `\^` + body[1:]
	}
	return prefix + body + "]"
}
