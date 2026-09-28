package viewgen

// A bool's jsonsrc.Node text, a J9 text reference's member, and strconv's integer base.
const (
	litTrue     = "true"
	litFalse    = "false"
	memberText  = "text"
	decimalBase = 10
)

// tagJSON, tagOmitzero, tagSep and tagSkip read a vm struct field's `json` tag.
const (
	tagJSON     = "json"
	tagOmitzero = "omitzero"
	tagSep      = ","
	tagSkip     = "-"
)

// A number's checked precision, J10's safe-integer text, and a RawMessage's throwaway file name.
const (
	floatBits      = 64
	maxSafeIntText = "9007199254740991"
	rawSourceName  = "default.json"
)
