package canon

import "github.com/fantasim/canonlang/internal/diag"

// Severity of a finding (API.md §4.1).
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// ValueKind is the kind of a Value (API.md §5.2).
type ValueKind string

const (
	KindBool      ValueKind = "bool"
	KindInt       ValueKind = "int"
	KindFloat     ValueKind = "float"
	KindString    ValueKind = "string"
	KindDuration  ValueKind = "duration"
	KindEnum      ValueKind = "enum"
	KindRecord    ValueKind = "record"
	KindVariant   ValueKind = "variant"
	KindList      ValueKind = "list"
	KindKeyedList ValueKind = "keyedList"
	KindTable     ValueKind = "table"
	KindMap       ValueKind = "map"
	KindRef       ValueKind = "ref"
	KindNone      ValueKind = "none"
	KindRange     ValueKind = "range"
	KindAsset     ValueKind = "asset"
)

// OriginKind says where a value comes from, one per provenance of EVALUATION.md §13.
type OriginKind string

const (
	OriginLiteral  OriginKind = "literal"  // a literal in a .canon source
	OriginJSON     OriginKind = "json"     // a node of a loaded JSON file
	OriginCSV      OriginKind = "csv"      // a cell of a loaded CSV file
	OriginDefines  OriginKind = "defines"  // a #define read by load.defines
	OriginText     OriginKind = "text"     // a file read by load.text
	OriginDefault  OriginKind = "default"  // a field default
	OriginSpread   OriginKind = "spread"   // copied by a spread
	OriginComputed OriginKind = "computed" // built by an expression
	OriginLayer    OriginKind = "layer"    // set by an amendment of a layer
)

// EditMode says how a value can be edited (rule W4).
type EditMode string

const (
	EditCanon EditMode = "canon"
	EditJSON  EditMode = "json"
	EditNone  EditMode = "none"
)

// Reason says why a value is not editable, one per row of API.md §7.2.
type Reason string

const (
	ReasonNone     Reason = ""
	ReasonComputed Reason = "computed"
	ReasonLayered  Reason = "layered"
	ReasonFormat   Reason = "format"
	ReasonInput    Reason = "input"
	ReasonKey      Reason = "key"
	ReasonPseudo   Reason = "pseudo"
	ReasonOrder    Reason = "order"
	ReasonLayer    Reason = "layer"
)

// RefKind says how a reference names its target (rule R7).
type RefKind string

const (
	RefValue RefKind = "value"
	RefKey   RefKind = "key"
	RefCode  RefKind = "code"
	RefView  RefKind = "view"
	RefCheck RefKind = "check"
	RefLayer RefKind = "layer"
)

// OpKind names an operation: the "op" string of its JSON form (rule E24).
type OpKind string

const (
	OpSet      OpKind = "set"
	OpReset    OpKind = "reset"
	OpAdd      OpKind = "add"
	OpInsert   OpKind = "insert"
	OpAddEntry OpKind = "addEntry"
	OpRemove   OpKind = "remove"
	OpMove     OpKind = "move"
	OpRename   OpKind = "rename"
	OpRetire   OpKind = "retire"
	OpUnretire OpKind = "unretire"
	OpSetCase  OpKind = "setCase"
)

// None is the absent value of an optional (rule E7).
var None Lit = noneLit{}

// ChangeKind says what an edit did to a file (API.md §8.1).
type ChangeKind string

const (
	Modified ChangeKind = "modified"
	Created  ChangeKind = "created"
	Deleted  ChangeKind = "deleted"
	Renamed  ChangeKind = "renamed"
)

// EventCause says what triggered an Event (rule W15).
type EventCause string

const (
	CauseExternal EventCause = "external"
	CauseEdit     EventCause = "edit"
	CauseOverlay  EventCause = "overlay"
)

// Target is an output kind (SPEC §14).
type Target string

const (
	TargetGo   Target = "go"
	TargetCpp  Target = "cpp"
	TargetTS   Target = "ts"
	TargetJSON Target = "json"
	TargetView Target = "view"
)

// OutputStatus says what happened to an output file (API.md §13.1).
type OutputStatus string

const (
	OutputWritten   OutputStatus = "written"
	OutputUnchanged OutputStatus = "unchanged"
	OutputStale     OutputStatus = "stale"   // Check mode: would be written
	OutputAdopted   OutputStatus = "adopted" // taken over through BuildOptions.Adopt (rule B2)
)

// severities maps each Severity to the one diag writes (API.md §4.1).
var severities = map[Severity]diag.Severity{
	SeverityError:   diag.Error,
	SeverityWarning: diag.Warning,
}

// The versions Version reports (API.md §14, IMPLEMENTATION-PLAN.md §9).
const (
	compilerVersion   = "0.1.0"
	languageVersion   = "0.1"
	fingerprintFormat = "canon-fp v1"
	viewModelFormat   = "canon-vm/1"
	lockFormat        = "canon.lock v1"
	vcsRevisionKey    = "vcs.revision"
	vcsModifiedKey    = "vcs.modified"
	trueText          = "true"
	modulePath        = "github.com/fantasim/canonlang"
)

// A Go pseudo-version: its time stamp and the revision prefix after it (go help modules).
const (
	pseudoSep       = "-"
	pseudoStampSeps = "-."
	pseudoTimeLen   = 14
	pseudoRevLen    = 12
	pseudoBits      = 64
	hexBase         = 16
	decimalBase     = 10
)

// Pieces of the error texts of rule X1.
const (
	textSep          = ": "
	fmtOpPrefix      = "op %d: "
	fmtCount         = "%d %s"
	textError        = "error"
	textErrors       = "errors"
	fmtFindingAt     = "%s:%d:%d: %s"
	textListSep      = ", "
	textExpected     = "expected "
	textGot          = ", got "
	fmtDecode        = "view model %s: %w"
	fmtDecodeFinding = "finding: %w"
	fmtSeverity      = "%w %q"
	msgUnimplemented = "unimplemented"
)
