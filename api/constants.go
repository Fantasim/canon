package canon

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/value"
)

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
	ReasonBroken   Reason = "broken"
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
	// OpRenameName renames a Canon name (API.md §8.9).
	OpRenameName OpKind = "renameName"
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

// originKinds names each provenance kind as OriginKind does, indexed by value.ProvKind (EVALUATION.md §13).
var originKinds = [...]OriginKind{
	value.ProvLiteral: OriginLiteral, value.ProvJSON: OriginJSON, value.ProvCSV: OriginCSV,
	value.ProvDefines: OriginDefines, value.ProvText: OriginText, value.ProvDefault: OriginDefault,
	value.ProvSpread: OriginSpread, value.ProvComputed: OriginComputed, value.ProvLayer: OriginLayer,
}

// editModes and reasons are edit's answers in the API's words, indexed by edit's (API.md §7).
var (
	editModes = [...]EditMode{edit.ModeNone: EditNone, edit.ModeCanon: EditCanon, edit.ModeJSON: EditJSON}
	reasons   = [...]Reason{
		edit.ReasonNone: ReasonNone, edit.ReasonComputed: ReasonComputed, edit.ReasonLayered: ReasonLayered,
		edit.ReasonFormat: ReasonFormat, edit.ReasonInput: ReasonInput, edit.ReasonKey: ReasonKey,
		edit.ReasonPseudo: ReasonPseudo, edit.ReasonOrder: ReasonOrder, edit.ReasonLayer: ReasonLayer,
		edit.ReasonBroken: ReasonBroken,
	}
)

// opKinds names each of edit's operations as the API does, indexed by edit's (rule E24).
var opKinds = [...]OpKind{
	edit.OpSet: OpSet, edit.OpReset: OpReset, edit.OpAdd: OpAdd, edit.OpInsert: OpInsert,
	edit.OpAddEntry: OpAddEntry, edit.OpRemove: OpRemove, edit.OpMove: OpMove, edit.OpRename: OpRename,
	edit.OpRetire: OpRetire, edit.OpUnretire: OpUnretire, edit.OpSetCase: OpSetCase, edit.OpRenameName: OpRenameName,
}

// opUnknown is an Op kind the API does not name, which edit refuses (rule E2).
const opUnknown = edit.OpRenameName + 1

// changeKinds names each kind of file change as the API does, indexed by edit's; a removed
// directory is no FileChange (rule N6).
var changeKinds = [...]ChangeKind{
	edit.ChangeModified: Modified, edit.ChangeCreated: Created, edit.ChangeDeleted: Deleted,
	edit.ChangeRenamed: Renamed, edit.ChangeRemovedDir: "",
}

// refKinds names each of edit's reference kinds as the API does, indexed by edit's (rule R7).
var refKinds = [...]RefKind{
	edit.RefValue: RefValue, edit.RefKey: RefKey, edit.RefCode: RefCode,
	edit.RefView: RefView, edit.RefCheck: RefCheck, edit.RefLayer: RefLayer,
}

// Paths as Value reads them: a lone segment's root, a segment's first bytes, error details (API.md §6.1).
const (
	segmentRoot       = "v"
	pathSegmentStarts = ".["
	fmtAtByte         = "%v at byte %d"
	fmtSegment        = "segment %s"
	fmtRoot           = "root %s"
)

// wireSchema is the `$schema` of the document Value.JSON reads a value's wire form back from.
const wireSchema = "canon.value@00000000"

// severities maps each Severity to the one diag writes (API.md §4.1).
var severities = map[Severity]diag.Severity{
	SeverityError:   diag.Error,
	SeverityWarning: diag.Warning,
}

// The versions Version reports (API.md §14, IMPLEMENTATION-PLAN.md §9).
const (
	languageVersion   = "0.1"
	fingerprintFormat = "canon-fp v1"
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
	fmtAtPointer     = "%w at %q"
	fmtMemberCase    = "%w at %q: the field is %q"
	fmtIndexPointer  = "%s/%d"
	fmtTypeVM        = "type %s: %w"
	fmtDecodeFinding = "finding: %w"
	fmtSeverity      = "%w %q"
	fmtWrap          = "%w: %w"
	fmtEncodeJSON    = "%s: %w"
	typeEvalResult   = "EvalResult"
	typeHeading      = "Heading"
	fmtUnknown       = "%w: %s"
	fmtMixed         = "%w: %s: %w"
	fmtQuoted        = "%q"
	expectedTargets  = "go, cpp, ts, json or view"
	expectedPattern  = "an RE2 regular expression"
)

// What ViewModel.Decode reads: pointers, struct tags, a neutral text reference's one member (J9).
const (
	pointerSep    = "/"
	pointerTilde  = "~"
	structTagJSON = "json"
	tagSep        = ","
	tagSkip       = "-"
	textRefMember = "text"
)

var (
	pointerEscaper = strings.NewReplacer(pointerTilde, "~0", pointerSep, "~1")
	lineEnd        = []byte("\n")
)
