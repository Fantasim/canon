package i18n

// Text is a key's text in lang, and whether it fell back to the source language (I18N.md B1):
// the source text is used wherever lang has no non-empty translation for key.
type Text struct {
	Value    string
	Fallback bool
}

// Fallback is key's text in lang: cat's source text, else lang's translation when it has a
// non-empty one (I18N.md B1). The source language itself never falls back.
func Fallback(cat *Catalogue, lang *Language, key string) Text {
	entry, ok := cat.Lookup(key)
	if !ok {
		return Text{}
	}
	if lang != nil {
		if t, ok := lang.Texts[key]; ok {
			return Text{Value: t}
		}
	}
	return Text{Value: entry.Text, Fallback: lang != nil}
}
