package render

// dot joins the segments of a key.
const dot = "."

// noneText is how `none` renders without a field `none` text (VIEWMODEL.md X4).
const noneText = "none"

// The disambiguation of equal titles (VIEWMODEL.md S9): `<title> (<key>)`.
const (
	keyOpen  = " ("
	keyClose = ")"
)

// SharedTitle is how many entries of one collection share a title that S9 disambiguates.
const SharedTitle = 2
