package edit

import (
	"context"
	"io/fs"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
)

// The segment forms of API.md §6.1.
const (
	SegField, SegKey, SegPos SegKind = 0, 1, 2
)

// The key forms of API.md §6.1.
const (
	KeyWord, KeyInt, KeyString KeyLitKind = 0, 1, 2
)

// Path syntax (API.md §6.1).
const (
	fieldMark    = '.'
	bracketOpen  = '['
	bracketClose = "]"
	positionMark = "#"
	packageMark  = ":"
	jsonQuote    = `"`
	minus        = '-'
	zeroDigit    = '0'
	underscore   = "_"
	decimalBase  = 10
	int64Bits    = 64
)

// fmtSyntaxError is a SyntaxError's text: the sentinel, the reason, the offset.
const fmtSyntaxError = "%v: %v at byte %d"

// The pseudo-fields of API.md P3.
const (
	pseudoID      = "id"
	pseudoRetired = "retired"
	pseudoKind    = "kind"
)

// The editing modes of API.md W4; ModeNone is a value no edit can reach.
const (
	ModeNone, ModeCanon, ModeJSON Mode = 0, 1, 2
)

// The rows of API.md §7.2, in table order: the first that applies names the reason.
const (
	ReasonNone, ReasonComputed, ReasonLayered, ReasonFormat, ReasonInput Reason = 0, 1, 2, 3, 4
	ReasonKey, ReasonPseudo, ReasonOrder, ReasonLayer, ReasonBroken      Reason = 5, 6, 7, 8, 9
)

// The operations of API.md §8.3.
const (
	OpSet, OpReset, OpAdd, OpInsert, OpAddEntry, OpRemove           Op = 0, 1, 2, 3, 4, 5
	OpMove, OpRename, OpRetire, OpUnretire, OpSetCase, OpRenameName Op = 6, 7, 8, 9, 10, 11
)

// Where a value's source stands while a path is walked from its root (API.md W3).
const (
	stTree state = iota
	stAbsent
	stSpread
	stComputed
	stFormat
	stLayered
	stOpaque // an amendment's expression: replaced whole, nothing below it is a source
)

// keyDepth is how many steps a key field lies below its keyed list: the element, then the field.
// stableDepth is how many a root stable table's @stable field lies below it: the entry, the field.
const (
	keyDepth    = 2
	stableDepth = 2
)

// layerFileSuffix names a layer file W11 creates: <package dir>/<layer>.layer.canon.
const layerFileSuffix = ".layer.canon"

// rootMark starts a path written with a root, `@root/rest` (WIRE.md §2.1).
const rootMark = '@'

// pathSep separates the directories of a display path.
const pathSep = "/"

// fmtPathError is a PathError's text: the sentinel, the segment.
const fmtPathError = "%v at segment %d"

// rootSeg is PathError.Seg for a failure at the root.
const rootSeg = -1

// steppers moves a walk one step from each state (API.md W3).
var steppers = [...]func(*judge, cursor, int) cursor{
	stTree:     treeStep,
	stAbsent:   absentStep,
	stSpread:   leave,
	stComputed: stay,
	stFormat:   stay,
	stLayered:  stay,
	stOpaque:   leave,
}

// rows are API.md §7.2's rows in table order: the first that applies names the reason.
var rows = [...]func(*judge) Reason{computedRow, layeredRow, formatRow, starRow, inputRow, keyRow, pseudoRow, orderRow, layerRow}

// itemOps take out, move or rename an item of their target's collection (starRow).
var itemOps = map[Op]bool{OpRemove: true, OpMove: true, OpRename: true}

// valueOps give a value back in place, which a region's restore replaces (log-2026-09-29 U-E22-r).
var valueOps = map[Op]bool{OpSet: true, OpReset: true}

// The constructors a ValueError names what was given by (API.md §8.2).
const (
	nameBool    = "Bool"
	nameInt     = "Int"
	nameFloat   = "Float"
	nameStr     = "Str"
	nameDur     = "Dur"
	nameMember  = "Member"
	nameKey     = "Key"
	nameIntKey  = "IntKey"
	namePathKey = "PathKey"
	nameCase    = "Case"
	callOpen    = "("
	callClose   = ")"
)

// form is the shape of a Lit that typing dispatches on; formOther is a scalar.
type form uint8

// The Lit forms typing dispatches on.
const (
	formOther form = iota
	formNone
	formList
	formObj
	formMap
	formJSON
	formSource
)

// litNames name the forms describe writes without an argument.
var litNames = [...]string{
	formNone: "None", formList: "List", formObj: "Obj", formMap: "Map", formJSON: "FromJSON", formSource: "Source",
}

// Number texts.
const (
	floatFormat  = 'g'
	shortestPrec = -1
	float32Bits  = 32
	exponentMark = "e"
)

// The details of a ValueError (API.md V1).
const (
	fmtValueError                  = "%v: expected %s, got %s"
	detailAt                       = "at "
	detailSep                      = ": "
	detailFraction                 = "holds a fraction of a millisecond"
	detailUnit                     = "its JSON source counts it in whole "
	detailNotFinite                = "not a finite number"
	detailDependent                = "a type computed from a value takes a name, or a literal one of its branches takes"
	detailNotNamed                 = "names nothing in the type its record computes here"
	detailField                    = "no such field: "
	detailTwice                    = "field given twice: "
	detailNotLiteral               = "not a Canon literal"
	detailNotContextual            = "not a contextual name"
	detailRange                    = "integer out of range"
	detailNameKey                  = "a bare name keys only an enum or a ref"
	detailUndecoded                = "not decodable"
	detailInput                    = "an input field has no value to give: "
	detailDeep                     = "nested too deep"
	detailSlots                    = "its JSON source has parallel key slots for at most "
	detailSlotField                = "a slot of its JSON source writes both fields; no value for "
	maxLitDepth                    = 512
	maxUndoRounds, templateSegment = 8, `[^/.][^/]*` // an Undo's verifications; a templated field's path segment
)

// The files a FromJSON or a Source is read from; a Source is the value of a `let` (API.md §8.2).
const (
	jsonName     = "value.json"
	sourceName   = "value.canon"
	sourcePrefix = "package _v\n\nlet _v = "
)

// The kinds of Ref (API.md R7), in the order R8 sorts references of one span.
const (
	RefValue RefKind = iota
	RefKey
	RefCode
	RefView
	RefCheck
	RefLayer
)

// The kinds of Change (API.md §8.1).
const (
	ChangeModified ChangeKind = iota
	ChangeCreated
	ChangeDeleted
	ChangeRenamed
	ChangeRemovedDir // a directory left empty, Path its display path, never a FileChange (API.md N6)
)

// refusals are the errors Apply returns as they are; any other is an internal failure.
var refusals = [...]error{
	ErrBadPath, ErrNoPath, ErrAmbiguousPath, ErrNoValue, ErrNotAnalyzed, ErrForeign, ErrBadValue, ErrBadOp,
	ErrNoHost, ErrNotEditable, ErrKeyExists, ErrStableKey, ErrPathCollision, ErrNoProject, ErrInternal, ErrUnwritable,
	ErrNameClash, context.Canceled, context.DeadlineExceeded,
}

// reasonNames are the names of API.md §7.2's rows, which a NotEditableError prints.
var reasonNames = [...]string{
	ReasonNone: "none", ReasonComputed: "computed", ReasonLayered: "layered", ReasonFormat: "format",
	ReasonInput: "input", ReasonKey: "key", ReasonPseudo: "pseudo", ReasonOrder: "order", ReasonLayer: "layer",
	ReasonBroken: "broken",
}

// handlers apply each operation of API.md §8.3 to the current state.
var handlers = [...]func(*opCtx) error{
	OpSet: setOp, OpReset: resetOp, OpAdd: addOp, OpInsert: insertOp, OpAddEntry: addEntryOp,
	OpRemove: removeOp, OpMove: moveOp, OpRename: renameOp, OpRetire: retireOp,
	OpUnretire: unretireOp, OpSetCase: setCaseOp,
}

// Texts an edit writes: Canon literals (API.md M7), new files (N5, W11), JSON sources (M8).
const (
	space        = " "
	newline      = "\n"
	blankLine    = "\n\n"
	emptyBrace   = "{}"
	braceOpenSp  = "{ "
	braceCloseSp = " }"
	colonSp      = ": "
	dotSeg       = "."
	pointerSep   = "/"
	globMagic    = "*?[{"
	globBraces   = "{"
	tplOpen      = '{'
	tplClose     = '}'
)

// A load's `at:` option (WIRE.md 6.3, its grammar wire's); pointerFragment joins a file and a
// JSON pointer in it (RFC 6901 section 6).
const (
	loadAtOption    = "at"
	pointerFragment = "#"
)

// What a RenameName reads and writes (API.md E27, E33, E34; I18N.md K4; WIRE.md 5.5.2, 6.1).
const (
	fmtPosition, fmtLine, annotationMark     = "%s:%d:%d", "%s:%d", "@"
	jsonArgPath, jsonArgPairs, jsonArgInline = "path", "pairs", "inline"
)

// kindWords are I18N.md K4's kind words of fields and methods; caseStyles WIRE.md 5.5.2's
// styles; loadsData, codeTargets and dataModes what reads or writes data (API.md E34).
var (
	kindWords   = map[check.ObjKind]string{check.ObjField: syntax.WordField, check.ObjMethod: syntax.WordMethod}
	caseStyles  = map[string]caseStyle{"snake": {underscore, strings.ToLower}, "kebab": {"-", strings.ToLower}, "upper_snake": {underscore, strings.ToUpper}}
	loadsData   = map[string]bool{"dir": true, "csv": true}
	codeTargets = map[string]bool{check.TargetGo: true, check.TargetCpp: true, check.TargetTS: true}
	dataModes   = map[string]bool{check.ModeEmbedded: true, check.ModeData: true, check.ModeTypes: true}
)

// Keywords an edit writes, as the lexer spells them (GRAMMAR.md).
var (
	retiredWord = syntax.KwRetired.String()
	noneWord    = syntax.KwNone.String()
	packageWord = syntax.KwPackage.String()
	entryWord   = syntax.KwEntry.String()
	layerWord   = syntax.KwLayer.String()
	amendWord   = syntax.KwAmend.String()
)

// The one-field document an edit's value is encoded in to take its source wire (API.md M8).
const (
	wireSlot     = "v"
	wireSchema   = "canon.edit@00000000"
	wireValuePtr = "/value"
	// symPlaceholder starts the string a decoded symbol's token takes in the encoder (DECISIONS 175).
	symPlaceholder = "\x00canon symbol "
)

// holderBack is how far from a walk's end the cursor of the target's container is.
const holderBack = 2

// slotBack is how far from a pairs list element's cursor the record whose object writes its
// slot keys is (WIRE.md 5.14).
const slotBack = 2

// parentStepBack is how far from a path's end the step whose value holds the last one is.
const parentStepBack = 2

// regionKind is what a write may put in a region of the file before it (API.md M6).
type regionKind uint8

// What a region holds after a write: any text (a new file, a normalization), the text of one
// node (an item printed again), nothing (an item removed), one new item (an insertion point),
// a moved item's own lines (fromLo to fromHi), or a comma or none (after a kept neighbour).
const (
	regionAny, regionNode, regionGone, regionItem, regionMoved, regionComma regionKind = 0, 1, 2, 3, 4, 5
)

// cascadeStep is the operation index of the writes a cascade makes once every operation is
// applied (API.md E15).
const cascadeStep = -1

// pathMarks start a step inside a value in a finding's path (API.md §6.1).
const pathMarks = string(fieldMark) + string(bracketOpen)

// fitCodes are the findings at a field, or for E14 inside its value, that say the value does
// not fit its type: mismatches, refinements, `where`, assets, decoding (log-2026-09-29 M4 U4b-r3).
var fitCodes = map[diag.Code]bool{
	diag.E3802.Def().Code: true, diag.E3801.Def().Code: true, diag.E3002.Def().Code: true, diag.E3201.Def().Code: true,
	diag.E3202.Def().Code: true, diag.E3311.Def().Code: true, diag.E3403.Def().Code: true, diag.E3301.Def().Code: true,
	diag.E3302.Def().Code: true, diag.E3315.Def().Code: true, diag.E3204.Def().Code: true, diag.E3205.Def().Code: true,
	diag.E3206.Def().Code: true, diag.E3701.Def().Code: true, diag.E3702.Def().Code: true, diag.E3703.Def().Code: true,
	diag.E7110.Def().Code: true, diag.E7111.Def().Code: true, diag.E7112.Def().Code: true,
}

// fsErrors are the sentinels of a file system failure, which Apply returns as they are.
var fsErrors = [...]error{fs.ErrInvalid, fs.ErrPermission, fs.ErrExist, fs.ErrNotExist, fs.ErrClosed}

// A plain string literal's quote, escape and braces (GRAMMAR.md §2.6).
const (
	quoteMark  = jsonQuote
	escapeMark = `\`
	braceChars = emptyBrace
)

// Brackets and spaces the lines around a removed item are read by (FORMATTER.md §13 step 5).
const (
	openBrackets  = "{[("
	closeBrackets = "}])"
	spaceChars    = " \t"
)

// A file or a directory that exists only in an edit's memory.
const (
	opRead   = "read"
	fileMode = 0o644
	dirMode  = 0o755
)

// Texts of an edit's errors.
const (
	fmtNotEditable = "%v: reason %s"
	fmtCollision   = "%v: %s"
	fmtOpError     = "operation %d (%s): %v"
	fmtFile        = "%w: %s"
	fmtFileErr     = "%s: %w"
	fmtWrapped     = "%w: %w"
	fmtUnprinted   = "%w: %T"
	pathSegment    = "a value a path segment can hold"
	detailNotWord  = "a table key is a name"
	detailNotUTF8  = "text that is not UTF-8"
)

// The members of an operation's JSON form as bits of a set; `value` and `source` are one (E24).
const (
	mOp members = 1 << iota
	mPath
	mValue
	mKey
	mIndex
	mCase
	mName
)

// The entries of jsonMembers, in the order the JSON form writes them.
const (
	iOp = iota
	iPath
	iIndex
	iKey
	iCase
	iValue
	iSource
	iName
)

// Texts of the JSON form of an operation.
const (
	jsonOpen     = '{'
	jsonClose    = '}'
	jsonComma    = ','
	jsonColon    = ':'
	jsonNullByte = 'n' // no JSON value but null starts with it
	textObject   = "an object"
	listSep      = ", "
	fmtOpJSON    = "%w: %s"
	fmtOpJSONErr = "%w: %s: %w"
	fmtBadMember = "%w: %w"
)

// opNames are the `op` strings of API.md E24, by operation.
var opNames = [...]string{
	OpSet: "set", OpReset: "reset", OpAdd: "add", OpInsert: "insert", OpAddEntry: "addEntry", OpRemove: "remove",
	OpMove: "move", OpRename: "rename", OpRetire: "retire", OpUnretire: "unretire", OpSetCase: "setCase",
	OpRenameName: "renameName",
}

// opShapes are the members each operation takes and needs (API.md §8.3): SetCase's fields are optional.
var opShapes = [...]opShape{
	OpSet: {mValue, mValue}, OpReset: {}, OpAdd: {mValue, mValue}, OpInsert: {mIndex | mValue, mIndex | mValue},
	OpAddEntry: {mKey | mValue, mKey | mValue}, OpRemove: {}, OpMove: {mIndex, mIndex}, OpRename: {mKey, mKey},
	OpRetire: {}, OpUnretire: {}, OpSetCase: {mCase | mValue, mCase}, OpRenameName: {mName, mName},
}

// jsonMembers are the members of an operation's JSON form (API.md §8.8), in writing order.
var jsonMembers = [...]jsonMember{
	iOp: {"op", mOp, readOp}, iPath: {"path", mPath, readPath}, iIndex: {"index", mIndex, readIndex},
	iKey: {"key", mKey, readKey}, iCase: {"case", mCase, readCase}, iValue: {"value", mValue, readValue},
	iSource: {"source", mValue, readSource}, iName: {"name", mName, readName},
}

// recheckCodes are the wire findings about a value, not its static shape: the re-check reports
// them (API.md V2, V3; log-2026-09-29 M4 U4a).
var recheckCodes = map[diag.Code]bool{
	diag.E3302.Def().Code: true, diag.E3201.Def().Code: true, diag.E3202.Def().Code: true, diag.E3102.Def().Code: true,
	diag.E3317.Def().Code: true,
}

// The journal of a commit: .canon/journal/<hex of the new revision>.json under the project
// directory, the revision's scheme cut at revisionSep, no Windows name (API.md N10, S3).
const (
	revisionSep    = ':'
	JournalDir     = ".canon/journal"
	journalVersion = 1
)

// A file a commit stages beside its target before the rename: hidden, never a source (API.md N10).
const (
	stagePrefix = "."
	stageSuffix = ".canon-edit"
)

// Line ends of a written file (API.md N12).
const (
	lineEnd = "\n"
	crlfEnd = "\r\n"
)

// The one line Recover logs (API.md O5): how many journals it rolled back, how many files and
// directories it changed.
const (
	msgRecovered = "rolled back an unfinished edit"
	attrJournals = "journals"
	attrChanges  = "changes"
)

// newAbsent is a journal file's New when the commit removes it.
const newAbsent = "absent"

// Why a journal, or the commit that would write it, is refused (log-2026-09-29 M4 U4c-r, U4c-r3).
const (
	reasonVersion = "unknown version"
	reasonPlace   = "path outside the project and its roots"
	reasonMode    = "mode beyond read and write bits"
	reasonDigest  = "invalid digest"
	reasonAbsent  = "old content for an absent file"
	reasonChanged = "changed since the edit"
	reasonKind    = "not a file the edit API writes"
	reasonLink    = "reached through a symbolic link"
	reasonForeign = "written on another machine"
	reasonStray   = "not the edit's"
	reasonHidden  = "hidden path"
	reasonTwice   = "path named twice"
	reasonOrphan  = "created directory above no new file"
)

// What a journal may name (log-2026-09-29 M4 U4c-r3): no hidden segment, the lock beside
// sources and JSON files (API.md E20), read and write bits only for a file.
const (
	hiddenMark        = '.'
	lockExt           = ".lock"
	rwBits     uint32 = 0o666
)

// expanders turn a Change into the files a commit writes or removes, by kind (API.md §8.1, N8).
var expanders = [...]func(Change) []commitFile{
	ChangeModified:   modifiedFiles,
	ChangeCreated:    createdFiles,
	ChangeDeleted:    deletedFiles,
	ChangeRenamed:    renamedFiles,
	ChangeRemovedDir: removedDirFiles,
}

// findingSep joins the parts of a lock finding's identity, a byte no path or text holds (E23).
const findingSep = "\x00"
