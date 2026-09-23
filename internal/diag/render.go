package diag

import (
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/source"
)

// renderer renders templates and arguments with one file set (ERRORS.md §1.2, §1.3).
type renderer struct {
	files Files
}

// message renders a builder's template with its arguments.
func (r renderer) message(b *Builder) string {
	v := b.def.Variants[b.variant]
	return r.template(v.Template, v.Args, b.args)
}

// note renders a related note (ERRORS.md §1.5).
func (r renderer) note(n Note) string {
	if n.template == "" {
		return r.expr(n.decl)
	}
	return r.template(n.template, []Arg{{Name: noteNameArg, Type: ArgTypeName}}, []any{n.name})
}

// template reads tpl left to right: escapes first, then placeholders, then text.
func (r renderer) template(tpl string, params []Arg, args []any) string {
	var sb strings.Builder
	for i := 0; i < len(tpl); {
		if to, width, ok := escapeAt(tpl[i:]); ok {
			sb.WriteString(to)
			i += width
			continue
		}
		if tpl[i] != placeholderOpen {
			sb.WriteByte(tpl[i])
			i++
			continue
		}
		end := strings.IndexByte(tpl[i:], placeholderClose)
		if end < 0 {
			sb.WriteString(tpl[i:])
			break
		}
		sb.WriteString(r.placeholder(tpl[i+1:i+end], params, args))
		i += end + 1
	}
	return sb.String()
}

// escapeAt reports the rendering and width of the escape s starts with, if any.
func escapeAt(s string) (string, int, bool) {
	for _, e := range templateEscapes {
		if strings.HasPrefix(s, e.from) {
			return e.to, len(e.from), true
		}
	}
	return "", 0, false
}

// placeholder renders the argument a placeholder names; the generator guarantees it exists.
func (r renderer) placeholder(name string, params []Arg, args []any) string {
	for i, p := range params {
		if p.Name == name && i < len(args) {
			return r.arg(p.Type, args[i])
		}
	}
	return ""
}

// arg renders one argument by its type (ERRORS.md §1.3).
func (r renderer) arg(t ArgType, a any) string {
	switch t {
	case ArgTypeNames:
		return strings.Join(asStrings(a), listSep)
	case ArgTypeChain:
		return strings.Join(asStrings(a), chainSep)
	case ArgTypeType:
		return typeText(a)
	case ArgTypeTypes:
		return typesText(a)
	case ArgTypeValue:
		v, _ := a.(ValueArg)
		return valueText(v)
	case ArgTypeExpr:
		s, _ := a.(source.Span)
		return r.expr(s)
	case ArgTypeInt:
		n, _ := a.(int64)
		return strconv.FormatInt(n, decimalBase)
	case ArgTypeRune:
		c, _ := a.(rune)
		return runeText(c)
	case ArgTypeLoc:
		s, _ := a.(source.Span)
		return r.loc(s)
	case ArgTypePointer:
		p, _ := a.(string)
		return fragment(p)
	case ArgTypeKind:
		k, _ := a.(Kind)
		return k.Word()
	case ArgTypeMessage:
		m, _ := a.(Message)
		return r.nested(m)
	case ArgTypeName, ArgTypePath, ArgTypeText:
	}
	s, _ := a.(string)
	return s
}

func asStrings(a any) []string {
	s, _ := a.([]string)
	return s
}

func typeText(a any) string {
	if t, ok := a.(TypeArg); ok && t != nil {
		return t.String()
	}
	return ""
}

func typesText(a any) string {
	ts, _ := a.([]TypeArg)
	parts := make([]string, 0, len(ts))
	for _, t := range ts {
		parts = append(parts, typeText(t))
	}
	return strings.Join(parts, listSep)
}

func valueText(v ValueArg) string {
	if v == nil {
		return ""
	}
	return v.CanonText()
}

// nested renders another code's message with this renderer's files (E1703).
func (r renderer) nested(m Message) string {
	if m.finding == nil {
		return ""
	}
	return r.message(m.finding)
}

// runeText is U+ and at least four upper-case hexadecimal digits.
func runeText(c rune) string {
	digits := strings.ToUpper(strconv.FormatInt(int64(c), hexBase))
	if pad := runeHexWidth - len(digits); pad > 0 {
		digits = strings.Repeat(zeroDigit, pad) + digits
	}
	return runePrefix + digits
}

// expr is the source text of a span, each run of whitespace replaced by one space.
func (r renderer) expr(s source.Span) string {
	var sb strings.Builder
	inRun := false
	for _, c := range spanText(r.files.Content(s.File), s) {
		white := strings.IndexByte(whitespace, c) >= 0
		switch {
		case !white:
			sb.WriteByte(c)
		case !inRun:
			sb.WriteString(space)
		}
		inRun = white
	}
	return sb.String()
}

// spanText is the bytes of s within content, clamped to it.
func spanText(content []byte, s source.Span) []byte {
	end := min(max(int(s.End), 0), len(content))
	start := min(max(int(s.Start), 0), end)
	return content[start:end]
}

// loc is `<display path>:<line>` of the span's start (API.md F12).
func (r renderer) loc(s source.Span) string {
	line, _ := r.files.Position(s.File, s.Start)
	return r.files.Path(s.File) + locSep + strconv.Itoa(line)
}

// fragment writes an RFC 6901 pointer in its URI fragment form (RFC 6901 §6, RFC 3986 §3.5).
func fragment(pointer string) string {
	var sb strings.Builder
	sb.WriteString(fragmentMark)
	for i := 0; i < len(pointer); i++ {
		c := pointer[i]
		if isAlnum(c) || strings.IndexByte(fragmentSafe, c) >= 0 {
			sb.WriteByte(c)
			continue
		}
		sb.WriteByte(percent)
		sb.WriteByte(hexDigits[c>>hexShift])
		sb.WriteByte(hexDigits[c&hexLowMask])
	}
	return sb.String()
}

func isAlnum(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9'
}
