package build

// The revision of a snapshot: "r1:" and the SHA-256 of its read-set listing (API.md S3).
const (
	revisionPrefix = "r1:"
	listingSep     = "\x00"
	listingEnd     = "\n"
	unreadMark     = "unreadable" // for a file that cannot be read, in place of its SHA-256
	listingMark    = "."          // the file set's own listing, when it cannot be read
)

// fmtWrap wraps an error that already names its file.
const fmtWrap = "%w"
