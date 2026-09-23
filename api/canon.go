package canon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"time"
)

// ---------------------------------------------------------------------------
// Opening a project (API.md §2)
// ---------------------------------------------------------------------------

// Options configures a Project. The zero value is valid. See API.md §2.1.
type Options struct {
	// Layers to apply, in order (SPEC §19, CLI --layer).
	Layers []string
	// Lang is the language of translated texts (check messages, titles, show labels).
	// Empty means the project's source language.
	Lang string
	// EditLayer, when set, makes Set, Reset and AddEntry write amendments into this layer
	// instead of the base sources (API.md §7.5).
	EditLayer string
	// Roots replaces the directory of declared roots: name -> directory (CLI --root).
	// Relative directories are relative to the project root.
	Roots map[string]string
	// FS is the file system for every read and write. Nil means the operating system.
	FS FS
	// Cache is the cache directory. Empty means <root>/.canon/cache; "off" disables it.
	Cache string
	// Workers bounds parallelism. Zero means runtime.GOMAXPROCS(0). Results never depend on it.
	Workers int
	// MaxFindings is the number of findings kept per package. Zero means 1000.
	MaxFindings int
	// Logger receives diagnostics about the compiler itself, never findings. Nil discards.
	Logger *slog.Logger
}

// FS is the file system a Project reads and writes. Names are absolute and use '/'.
// ReadDir may return entries in any order: the compiler sorts them (API.md §2.2).
type FS interface {
	ReadFile(name string) ([]byte, error)
	Stat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	// WriteFile must be atomic: a reader sees the old or the new content, never a mix.
	WriteFile(name string, data []byte) error
	Rename(oldname, newname string) error
	Remove(name string) error
	MkdirAll(name string) error
}

// Project is an opened Canon project: its sources, caches and current snapshot.
// A Project is safe for concurrent use; edits are serialized (API.md §3.3).
type Project struct {
	root string
	opts Options
}

// FindProject returns the directory holding project.canon, searching dir and then each
// parent. It returns ErrNoProject when none is found (rule O1).
func FindProject(dir string) (root string, err error) {
	panic("unimplemented")
}

// Open opens the project whose project.canon is in root. It checks project.canon and scans
// the file set; it parses nothing else (rules O2-O5). Errors in project.canon are returned as
// a *ProjectError.
func Open(root string, opts Options) (*Project, error) {
	panic("unimplemented")
}

// Close releases the project and stops every Watch. Later calls return ErrClosed (rule O6).
func (p *Project) Close() error {
	panic("unimplemented")
}

// Root returns the absolute project root directory.
func (p *Project) Root() string { return p.root }

// PackageInfo describes one package of the project (API.md §5.5).
type PackageInfo struct {
	Name    string   // dotted package name
	Dir     string   // display path of the package directory
	Files   []string // sources, translations and layers, byte order
	Imports []string // imported packages, sorted
	Layers  []string // layer names with a file in this package, sorted
}

// Packages lists every package of the project, sorted by name.
func (p *Project) Packages(ctx context.Context) ([]PackageInfo, error) {
	panic("unimplemented")
}

// ---------------------------------------------------------------------------
// Snapshots and revisions (API.md §3)
// ---------------------------------------------------------------------------

// Revision identifies a snapshot of the files a project has read: "r1:" followed by the hex
// SHA-256 of the read-set listing (rule S3). The empty Revision disables staleness checks
// in an Edit (rule S6).
type Revision string

// Revision returns the revision of the current snapshot, after a refresh (rule S1).
func (p *Project) Revision() Revision {
	panic("unimplemented")
}

// SetOverlay replaces a file's content in memory without writing it (API.md §3.4). file is
// a display path or an absolute path.
func (p *Project) SetOverlay(file string, content []byte) error {
	panic("unimplemented")
}

// ClearOverlay removes the overlay of file, if any.
func (p *Project) ClearOverlay(file string) error {
	panic("unimplemented")
}

// ---------------------------------------------------------------------------
// Findings (API.md §4)
// ---------------------------------------------------------------------------

// Severity of a finding.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Span is a range in a file. Lines and columns are 1-based; columns count UTF-8 bytes; the
// end is exclusive (API.md §1.3). File is a display path ("@resource/..." or project-relative)
// and is empty when there is no location.
type Span struct {
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Col     int    `json:"col,omitempty"`
	EndLine int    `json:"endLine,omitempty"`
	EndCol  int    `json:"endCol,omitempty"`
}

// Related is another location of a finding, with a note.
type Related struct {
	Span
	Note string `json:"note"`
}

// Frame is one frame of a Canon call stack.
type Frame struct {
	Fn string `json:"fn"`
	Span
}

// Finding is a diagnostic: a compiler error or warning, or the result of a check or warn
// (DIAG-02, API.md §4.1). Its JSON form is fixed by rule F5.
type Finding struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Span
	// Pointer is an RFC 6901 pointer when File is a loaded JSON file.
	Pointer string `json:"pointer,omitempty"`
	// Package is the package the finding belongs to.
	Package string `json:"package"`
	// Path is the canonical value path, starting at the value name, without package (rule F1).
	Path string `json:"path,omitempty"`
	// Message is translated into Options.Lang when a translation exists.
	Message string `json:"message"`
	// Check is the name of the check or warn that produced the finding, if named.
	Check string `json:"check,omitempty"`
	// Layer is the layer whose amendment produced the value, if any.
	Layer   string    `json:"layer,omitempty"`
	Related []Related `json:"related,omitempty"`
	// Stack is the Canon call stack, innermost first, at most 16 frames.
	Stack []Frame `json:"stack,omitempty"`
	// Reads lists, for a one-line record check without "at", the fields its condition reads,
	// in first-use order, so the studio can highlight them (VIEWMODEL.md J15, Q5).
	Reads []string `json:"reads,omitempty"`
}

// MarshalJSON writes the finding with the key order and omissions of rule F5.
func (f Finding) MarshalJSON() ([]byte, error) {
	panic("unimplemented")
}

// UnmarshalJSON reads the form written by MarshalJSON.
func (f *Finding) UnmarshalJSON(data []byte) error {
	panic("unimplemented")
}

// Summary counts the findings of a result, including those dropped by truncation (rule F7).
type Summary struct {
	Errors    int          `json:"errors"`
	Warnings  int          `json:"warnings"`
	Packages  int          `json:"packages"`
	Truncated []Truncation `json:"truncated,omitempty"`
}

// Truncation reports findings that were counted but not kept for one package.
type Truncation struct {
	Package  string `json:"package"`
	Errors   int    `json:"errors"`
	Warnings int    `json:"warnings"`
}

// ---------------------------------------------------------------------------
// Reading (API.md §5)
// ---------------------------------------------------------------------------

// CheckResult is the result of Check and LockCheck.
type CheckResult struct {
	Revision Revision
	Packages []string  // selected packages, sorted
	Findings []Finding // sorted per rule F2
	Summary  Summary
	Duration time.Duration
}

// HasErrors reports whether the result holds at least one error finding.
func (r *CheckResult) HasErrors() bool { return r != nil && r.Summary.Errors > 0 }

// Check runs build phases 1-7 on the selected packages (CLI.md §2.2 selectors; none selects
// all) and returns their findings (rules R1-R3). Findings are not errors of the call.
func (p *Project) Check(ctx context.Context, packages ...string) (*CheckResult, error) {
	panic("unimplemented")
}

// LockCheck verifies canon.lock of the selected packages, evaluating only stable
// collections and their dependencies (rule B4, LCK-03).
func (p *Project) LockCheck(ctx context.Context, packages ...string) (*CheckResult, error) {
	panic("unimplemented")
}

// ValueKind is the kind of a Value.
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

// TypeInfo describes the type of a value.
type TypeInfo struct {
	// Expr is the canonical type text, with qualified names: "[resource.farm.Level]".
	Expr string
	// VM is the type in the view model encoding (VIEWMODEL.md).
	VM json.RawMessage
}

// OriginKind says where a value comes from (EVL-07).
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

// Origin records where a value comes from (API.md §5.2).
type Origin struct {
	Kind OriginKind
	Span
	Pointer string  // RFC 6901, for OriginJSON
	Layer   string  // for OriginLayer
	Via     *Origin // default: the literal or object that omitted the field; spread: the copied value's origin
	Stack   []Frame // for OriginComputed, at most 16 frames
}

// EditMode says how a value can be edited.
type EditMode string

const (
	EditCanon EditMode = "canon" // the edit writes a .canon source
	EditJSON  EditMode = "json"  // the edit writes a JSON source
	EditNone  EditMode = "none"  // not editable; see Reason
)

// Reason says why a value is not editable (API.md §7.2).
type Reason string

const (
	ReasonNone     Reason = ""
	ReasonComputed Reason = "computed" // the path leaves the source tree
	ReasonLayered  Reason = "layered"  // set by an active layer other than EditLayer
	ReasonFormat   Reason = "format"   // csv, defines or text source
	ReasonInput    Reason = "input"    // an input field
	ReasonKey      Reason = "key"      // a key; use Rename
	ReasonPseudo   Reason = "pseudo"   // .id, .retired, .kind
	ReasonOrder    Reason = "order"    // order comes from file paths
	ReasonLayer    Reason = "layer"    // op cannot be written as an amendment
)

// Editability says whether and where a value can be edited.
type Editability struct {
	Mode   EditMode
	Reason Reason
	File   string // display path of the file an edit would write
	Origin string // for ReasonComputed: canonical path of the nearest editable source, or ""
	Layer  string // for ReasonLayered
}

// Value is a value of a snapshot, with its type and origin (API.md §5.2). It stays valid and
// unchanged after later edits (rule R4).
type Value struct {
	Path     string // canonical, package-qualified
	Kind     ValueKind
	Type     TypeInfo
	Text     string // canonical Canon text form
	Origin   Origin
	Editable Editability
}

// Value returns the final value at path (rules R4-R6).
func (p *Project) Value(ctx context.Context, path string) (*Value, error) {
	panic("unimplemented")
}

// String returns the canonical Canon text of the value.
func (v *Value) String() string { return v.Text }

// Bool returns the value and true if the value is a Bool.
func (v *Value) Bool() (bool, bool) { panic("unimplemented") }

// Int returns the value and true if the value is of an integer type.
func (v *Value) Int() (int64, bool) { panic("unimplemented") }

// Float returns the value and true if the value is a Float or Float32.
func (v *Value) Float() (float64, bool) { panic("unimplemented") }

// Str returns the value and true if the value is a String or an asset.
func (v *Value) Str() (string, bool) { panic("unimplemented") }

// Dur returns the value and true if the value is a Duration.
func (v *Value) Dur() (time.Duration, bool) { panic("unimplemented") }

// Member returns the enum's qualified name and the member's Canon name.
func (v *Value) Member() (enum, name string, ok bool) { panic("unimplemented") }

// Case returns the current case of a variant.
func (v *Value) Case() (string, bool) { panic("unimplemented") }

// Key returns the key of a ref, table entry or keyed-list element, in canonical text.
func (v *Value) Key() (string, bool) { panic("unimplemented") }

// IsNone reports whether the value is none.
func (v *Value) IsNone() bool { return v.Kind == KindNone }

// Len returns the number of elements of a list, table or map, the number of fields of a
// record or case, and 0 otherwise.
func (v *Value) Len() int { panic("unimplemented") }

// Children returns fields in declaration order (absent optional fields as none), elements in
// order, and map entries in insertion order.
func (v *Value) Children() []*Value { panic("unimplemented") }

// Child returns one child by a path segment: ".f", "[k]" or "[#n]".
func (v *Value) Child(seg string) (*Value, error) { panic("unimplemented") }

// JSON returns the wire form of the value (WIRE.md).
func (v *Value) JSON() []byte { panic("unimplemented") }

// RefKind says how a reference names its target (API.md §5.3).
type RefKind string

const (
	RefValue RefKind = "value" // a ref value holding the key
	RefKey   RefKind = "key"   // a map key of ref type
	RefCode  RefKind = "code"  // a name in a function body, default or constant
	RefView  RefKind = "view"  // a name in a view
	RefCheck RefKind = "check" // a name in a check
	RefLayer RefKind = "layer" // a name in an amendment
)

// Ref is one place that references an entry or member.
type Ref struct {
	Kind    RefKind
	Package string
	Path    string // canonical path of the referring value or map entry, for RefValue and RefKey
	Span
}

// RefsResult lists every reference to Target, in the order of rule F2.
type RefsResult struct {
	Target string
	Refs   []Ref
}

// Refs lists every place that references the table entry, keyed-list element or enum
// member at path (rules R7, R8).
func (p *Project) Refs(ctx context.Context, path string) (*RefsResult, error) {
	panic("unimplemented")
}

// ViewModel is the view model of one package (SPEC §16.10, VIEWMODEL.md).
type ViewModel struct {
	Package  string
	Revision Revision
	data     []byte
}

// JSON returns exactly the bytes emit view would write for the package (rule R9).
func (vm *ViewModel) JSON() []byte { return vm.data }

// Decode unmarshals the view model into v, typically a *vm.ViewModel (rule R10).
func (vm *ViewModel) Decode(v any) error { return json.Unmarshal(vm.data, v) }

// ViewModel returns the view model of pkg, even when the package has errors (rule R9).
func (p *Project) ViewModel(ctx context.Context, pkg string) (*ViewModel, error) {
	panic("unimplemented")
}

// ---------------------------------------------------------------------------
// Values in operations (API.md §8.2, API-01)
// ---------------------------------------------------------------------------

// Lit is a value given to an edit operation. The set of implementations is closed: use the
// constructors below. Values are checked against the type expected at the path before
// anything is written (rules V1-V4).
type Lit interface {
	isLit()
}

type (
	boolLit   struct{ v bool }
	intLit    struct{ v int64 }
	floatLit  struct{ v float64 }
	strLit    struct{ v string }
	durLit    struct{ v time.Duration }
	memberLit struct{ name string }
	keyLit    struct{ key string }
	intKeyLit struct{ key int64 }
	caseLit   struct {
		name   string
		fields Obj
	}
	noneLit   struct{}
	listLit   struct{ elems []Lit }
	mapLit    struct{ entries []KV }
	jsonLit   struct{ raw json.RawMessage }
	sourceLit struct{ text string }
)

func (boolLit) isLit()   {}
func (intLit) isLit()    {}
func (floatLit) isLit()  {}
func (strLit) isLit()    {}
func (durLit) isLit()    {}
func (memberLit) isLit() {}
func (keyLit) isLit()    {}
func (intKeyLit) isLit() {}
func (caseLit) isLit()   {}
func (noneLit) isLit()   {}
func (listLit) isLit()   {}
func (mapLit) isLit()    {}
func (jsonLit) isLit()   {}
func (sourceLit) isLit() {}
func (Obj) isLit()       {}

// Obj is a record, case or table entry value: fields by Canon name. Omitted fields keep
// their defaults (rule V3). Field order does not matter: fields are written in declaration
// order.
type Obj map[string]Lit

// KV is one entry of a map value.
type KV struct {
	Key   Lit
	Value Lit
}

// None is the absent value of an optional.
var None Lit = noneLit{}

// Bool is a Bool value.
func Bool(b bool) Lit { return boolLit{b} }

// Int is an integer value, accepted by every integer type and by Float.
func Int(n int64) Lit { return intLit{n} }

// Float is a Float or Float32 value. NaN and infinities are refused (E3202).
func Float(f float64) Lit { return floatLit{f} }

// Str is a String value; also accepted by asset types and string literal unions.
func Str(s string) Lit { return strLit{s} }

// Dur is a Duration value. It must be a whole number of milliseconds.
func Dur(d time.Duration) Lit { return durLit{d} }

// Member is an enum member, by Canon name (not wire value).
func Member(name string) Lit { return memberLit{name} }

// Key is a string key: the target of a ref, a table key, or a String map key.
func Key(k string) Lit { return keyLit{k} }

// IntKey is an integer key: a ref into a collection keyed by an integer field (RES-09), or
// an integer map key.
func IntKey(n int64) Lit { return intKeyLit{n} }

// Case is a variant case with its fields; fields may be nil for a case without fields.
func Case(name string, fields Obj) Lit { return caseLit{name, fields} }

// List is a list value, for [T] and keyed lists.
func List(elems ...Lit) Lit { return listLit{elems} }

// Map is a map value; entries keep the given order.
func Map(entries ...KV) Lit { return mapLit{entries} }

// FromJSON is a value in its wire form (WIRE.md), decoded against the expected type.
func FromJSON(raw []byte) Lit { return jsonLit{json.RawMessage(raw)} }

// Source is a value written as a Canon literal ("item { define: II_GEN_GOLD }"), parsed
// and typed against the expected type. Only literals and contextual names are allowed.
func Source(text string) Lit { return sourceLit{text} }

// ---------------------------------------------------------------------------
// Operations (API.md §8.3)
// ---------------------------------------------------------------------------

// OpKind names an operation. The values are the "op" strings of the JSON form (rule E24).
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

// Op is one operation of an Edit. Build it with the constructors below.
type Op struct {
	Kind  OpKind
	Path  string // value path (API.md §6)
	Value Lit    // Set, Add, Insert, AddEntry; SetCase fields (an Obj, or nil)
	Key   Lit    // AddEntry key; Rename new key
	Index int    // Insert, Move
	Case  string // SetCase
}

// Set replaces the value at path. A value equal to the field's default removes the field
// (rule E6); None follows rule E7.
func Set(path string, v Lit) Op { return Op{Kind: OpSet, Path: path, Value: v} }

// Reset removes the field at path from its literal or JSON object, so it takes its default.
func Reset(path string) Op { return Op{Kind: OpReset, Path: path} }

// Add appends v to the list or keyed list at path.
func Add(path string, v Lit) Op { return Op{Kind: OpAdd, Path: path, Value: v} }

// Insert inserts v at position index of the list or keyed list at path.
func Insert(path string, index int, v Lit) Op {
	return Op{Kind: OpInsert, Path: path, Index: index, Value: v}
}

// AddEntry adds the entry key with value v to the table or map at path.
func AddEntry(path string, key, v Lit) Op {
	return Op{Kind: OpAddEntry, Path: path, Key: key, Value: v}
}

// Remove removes the list element, map entry or non-stable table entry at path.
func Remove(path string) Op { return Op{Kind: OpRemove, Path: path} }

// Move moves the element or entry at path to position index among its siblings.
func Move(path string, index int) Op { return Op{Kind: OpMove, Path: path, Index: index} }

// Rename changes the key of the entry, element or map entry at path, and every reference to
// it, in one edit (API.md §8.4).
func Rename(path string, newKey Lit) Op { return Op{Kind: OpRename, Path: path, Key: newKey} }

// Retire marks the stable entry or @codes member at path as retired (SPEC §12).
func Retire(path string) Op { return Op{Kind: OpRetire, Path: path} }

// Unretire always fails with ErrStableKey: a retired id comes back only through a reviewed hand
// edit of canon.lock (LOCK.md §4.6, E6002). The op exists so that the refusal is explicit.
func Unretire(path string) Op { return Op{Kind: OpUnretire, Path: path} }

// SetCase changes the case of the variant at path, keeping compatible fields and applying
// fields on top (rule E14). Dropped fields are reported in EditResult.Dropped.
func SetCase(path, caseName string, fields Obj) Op {
	op := Op{Kind: OpSetCase, Path: path, Case: caseName}
	if fields != nil {
		op.Value = fields
	}
	return op
}

// MarshalJSON writes the JSON form of rules E24-E26.
func (o Op) MarshalJSON() ([]byte, error) { panic("unimplemented") }

// UnmarshalJSON reads the JSON form of rules E24-E26.
func (o *Op) UnmarshalJSON(data []byte) error { panic("unimplemented") }

// ---------------------------------------------------------------------------
// Editing (API.md §8-§10)
// ---------------------------------------------------------------------------

// Edit is an atomic list of operations (API.md §8.1). Its JSON form is §8.8.
type Edit struct {
	Base        Revision `json:"base"`
	Ops         []Op     `json:"ops"`
	AllowErrors bool     `json:"allowErrors,omitempty"`
	DryRun      bool     `json:"dryRun,omitempty"`
	Normalize   bool     `json:"normalize,omitempty"`
	Evaluate    []string `json:"evaluate,omitempty"`
}

// ChangeKind says what an edit did to a file.
type ChangeKind string

const (
	Modified ChangeKind = "modified"
	Created  ChangeKind = "created"
	Deleted  ChangeKind = "deleted"
	Renamed  ChangeKind = "renamed"
)

// FileChange is one file an edit wrote (or, with DryRun, would write).
type FileChange struct {
	Path    string
	Kind    ChangeKind
	OldPath string // for Renamed
	Before  []byte // DryRun only; nil when Created
	After   []byte // DryRun only; nil when Deleted
}

// Dropped is a value removed by a cascade (rules E14, E15).
type Dropped struct {
	Path  string          `json:"path"`  // canonical, package-qualified
	Value json.RawMessage `json:"value"` // wire form of the removed value
}

// EditResult is the result of Edit (API.md §8.1).
type EditResult struct {
	Applied  bool
	Revision Revision
	Changes  []FileChange // byte order of Path
	Findings []Finding    // affected packages, rule F2 order
	Summary  Summary
	Dropped  []Dropped
	Undo     []Op
	Eval     map[string]*EvalResult
}

// Edit applies e atomically: all operations or none (API.md §8-§10). When the result has
// error findings and AllowErrors is false, nothing is written and the error is a
// *RejectedError; the result is still returned (rule E19).
func (p *Project) Edit(ctx context.Context, e Edit) (*EditResult, error) {
	panic("unimplemented")
}

// ---------------------------------------------------------------------------
// Evaluate (API.md §11, VIEW-01)
// ---------------------------------------------------------------------------

// EvalRequest asks for the live view state of the value at Path, optionally with draft
// operations applied in memory.
type EvalRequest struct {
	Base  Revision `json:"base,omitempty"`  // optional; a stale Base is ErrStale
	Path  string   `json:"path"`            // a value; for a collection, every element gets a Heading (rule V6)
	Draft []Op     `json:"draft,omitempty"` // never written; same rules as Edit
	Lang  string   `json:"lang,omitempty"`  // overrides Options.Lang
}

// Text is an evaluated text.
type Text struct {
	Value    string `json:"value"`
	OK       bool   `json:"ok"`       // false when evaluation failed; the studio shows "—" (rule V11)
	Fallback bool   `json:"fallback"` // true when the source language was used instead of Lang
}

// ShowLine is one evaluated show line or view method (rule V10).
type ShowLine struct {
	Owner string `json:"owner"` // relative path of the value whose view declares the line
	Key   string `json:"key"`   // translation key: "Type.show.<id>" ("_<n>" when unnamed), or "Type.<method>"
	Label string `json:"label"`
	Text  Text   `json:"text"`
}

// Heading is how an element of a collection is listed (rule V6). Title is disambiguated
// when two elements of one collection render the same title (VIEWMODEL.md S9).
type Heading struct {
	Title    Text   `json:"title"`
	Subtitle Text   `json:"subtitle"`
	Preview  string `json:"preview"`
	Retired  bool   `json:"retired"`
	// Cells holds the rendered cell of each table column of mode "text", by field key.
	Cells map[string]Text `json:"cells"`
}

// EvalResult is the live view state of a value (API.md §11). Keys of When, Headings and
// Types are canonical paths relative to Path (rule V9).
type EvalResult struct {
	Revision Revision                   `json:"revision"`
	Path     string                     `json:"path"`
	Title    Text                       `json:"title"`
	Subtitle Text                       `json:"subtitle"`
	Preview  string                     `json:"preview"`
	When     map[string]bool            `json:"when"`
	Show     []ShowLine                 `json:"show"`
	Headings map[string]Heading         `json:"headings"`
	Types    map[string]json.RawMessage `json:"types"`
	Findings []Finding                  `json:"findings"`
	Summary  Summary                    `json:"summary"`
	Dropped  []Dropped                  `json:"dropped"`
}

// Evaluate computes titles, when conditions, show lines, dependent types and findings for
// the value at r.Path, with r.Draft applied in memory (rules V5-V14).
func (p *Project) Evaluate(ctx context.Context, r EvalRequest) (*EvalResult, error) {
	panic("unimplemented")
}

// ---------------------------------------------------------------------------
// Watching (API.md §12)
// ---------------------------------------------------------------------------

// EventCause says what triggered an Event.
type EventCause string

const (
	CauseExternal EventCause = "external"
	CauseEdit     EventCause = "edit"
	CauseOverlay  EventCause = "overlay"
)

// Event reports a new snapshot after a change and its automatic re-check.
type Event struct {
	Revision Revision
	Cause    EventCause
	Files    []string  // changed files, display paths, byte order
	Packages []string  // re-checked packages, sorted
	Findings []Finding // all current findings of Packages: replace, do not merge
	Summary  Summary
	Err      error
}

// Watch watches the project's files and calls fn after each coalesced change (rules
// W12-W16). It returns once watching has started; it stops when ctx is done.
func (p *Project) Watch(ctx context.Context, fn func(Event)) error {
	panic("unimplemented")
}

// ---------------------------------------------------------------------------
// Building and testing (API.md §13)
// ---------------------------------------------------------------------------

// Target is an output kind (SPEC §14).
type Target string

const (
	TargetGo   Target = "go"
	TargetCpp  Target = "cpp"
	TargetTS   Target = "ts"
	TargetJSON Target = "json"
	TargetView Target = "view"
)

// BuildOptions selects what Build does.
type BuildOptions struct {
	Packages []string // selectors; none selects all
	Targets  []Target // none selects all
	Check    bool     // write nothing; report what would change (CLI --check)
	// Adopt lists output paths the build may take over although they lack the generated-file
	// marker (CLI --adopt): the headers of @cpp(access: both) structs (rule B2).
	Adopt []string
}

// OutputStatus says what happened to an output file.
type OutputStatus string

const (
	OutputWritten   OutputStatus = "written"
	OutputUnchanged OutputStatus = "unchanged"
	OutputStale     OutputStatus = "stale"   // Check mode: would be written
	OutputAdopted   OutputStatus = "adopted" // taken over with --adopt (rule B2, GEN-05)
)

// Output is one emitted file.
type Output struct {
	Path    string
	Target  Target
	Package string
	Status  OutputStatus
}

// LockChange lists the canon.lock lines a build appended (or would append).
type LockChange struct {
	Package string
	File    string
	Lines   []string
}

// BuildResult is the result of Build.
type BuildResult struct {
	Check   *CheckResult
	Outputs []Output // byte order of Path
	Lock    []LockChange
	Stale   bool // Check mode: an output or a lock would change
}

// Build checks the selected packages and, without errors, writes their outputs and appends
// to their locks, as canon build does (rules B1, B2).
func (p *Project) Build(ctx context.Context, o BuildOptions) (*BuildResult, error) {
	panic("unimplemented")
}

// TestOptions selects tests.
type TestOptions struct {
	Packages []string
	Run      string // RE2 matched against test names; empty runs all
}

// ExpectFailure is one failing expect statement.
type ExpectFailure struct {
	Span
	Expect   string    // source text of the expect statement
	Expected string    // canonical text
	Got      string    // canonical text
	Findings []Finding // findings actually produced, for fails/warns
}

// TestCase is the result of one test block.
type TestCase struct {
	Package string
	Name    string
	Span
	Passed   bool
	Failures []ExpectFailure
}

// TestResult is the result of Test.
type TestResult struct {
	Tests    []TestCase // (package, file, line) order
	Passed   int
	Failed   int
	Duration time.Duration
}

// Test runs the test blocks of the selected packages (rule B3).
func (p *Project) Test(ctx context.Context, o TestOptions) (*TestResult, error) {
	panic("unimplemented")
}

// ---------------------------------------------------------------------------
// Formatting and version (API.md §14)
// ---------------------------------------------------------------------------

// Format returns the canonical layout of one .canon file (rule T1).
func Format(filename string, src []byte) ([]byte, error) {
	panic("unimplemented")
}

// FormatJSONSource returns the canonical source layout of a JSON file, preserving key
// order (rule T2, FMT-02).
func FormatJSONSource(src []byte) ([]byte, error) {
	panic("unimplemented")
}

// VersionInfo describes the compiler and the formats it produces (NFR-03).
type VersionInfo struct {
	Compiler    string   // semver of the compiler
	Languages   []string // supported language versions, ascending
	Fingerprint string   // "canon-fp v1"
	ViewModel   string   // "canon-vm/1"
	Lock        string   // "canon.lock v1"
	Commit      string   // VCS revision of the build, "" if unknown
}

// Version returns the compiler's version information.
func Version() VersionInfo {
	panic("unimplemented")
}

// ---------------------------------------------------------------------------
// Errors (API.md §15)
// ---------------------------------------------------------------------------

// Sentinel errors. Every error type below wraps exactly one of them; use errors.Is.
var (
	ErrNoProject          = errors.New("no project.canon found")
	ErrProject            = errors.New("invalid project")
	ErrUnsupportedVersion = errors.New("unsupported language version")
	ErrUnknownPackage     = errors.New("unknown package")
	ErrUnknownLayer       = errors.New("unknown layer")
	ErrBadPath            = errors.New("invalid path")
	ErrNoPath             = errors.New("no value at path")
	ErrAmbiguousPath      = errors.New("ambiguous path")
	ErrNoValue            = errors.New("value could not be computed")
	ErrBadOp              = errors.New("operation not valid here")
	ErrBadValue           = errors.New("value does not fit the type")
	ErrKeyExists          = errors.New("key already exists")
	ErrStableKey          = errors.New("stable id cannot be removed, renamed or un-retired")
	ErrNotEditable        = errors.New("value is not editable")
	ErrStale              = errors.New("sources changed since the base revision")
	ErrRejected           = errors.New("edit rejected: it produces errors")
	ErrNotCanonical       = errors.New("file is not in canonical layout")
	ErrPathCollision      = errors.New("file already exists")
	ErrOverlay            = errors.New("file has an unsaved overlay")
	ErrClosed             = errors.New("project is closed")
	ErrInternal           = errors.New("internal compiler error")
)

func opPrefix(op int) string {
	if op < 0 {
		return ""
	}
	return fmt.Sprintf("op %d: ", op)
}

func errText(op int, path string, sentinel error, detail string) string {
	s := opPrefix(op)
	if path != "" {
		s += path + ": "
	}
	s += sentinel.Error()
	if detail != "" {
		s += ": " + detail
	}
	return s
}

// ProjectError reports errors in project.canon or in Options (rule O3). Err is ErrProject or
// ErrUnsupportedVersion.
type ProjectError struct {
	Err      error
	Findings []Finding
}

func (e *ProjectError) Error() string {
	if len(e.Findings) > 0 {
		return e.Err.Error() + ": " + e.Findings[0].Message
	}
	return e.Err.Error()
}
func (e *ProjectError) Unwrap() error { return e.Err }

// PathError reports a problem with a path or an operation. Err is one of ErrBadPath,
// ErrNoPath, ErrAmbiguousPath, ErrNoValue, ErrBadOp, ErrKeyExists, ErrStableKey,
// ErrPathCollision, ErrOverlay.
type PathError struct {
	Op     int // index in Edit.Ops, or -1
	Path   string
	Err    error
	Detail string
	// Findings explain ErrNoValue; Candidates list qualified roots for ErrAmbiguousPath.
	Findings   []Finding
	Candidates []string
}

func (e *PathError) Error() string { return errText(e.Op, e.Path, e.Err, e.Detail) }
func (e *PathError) Unwrap() error { return e.Err }

// ValueError reports an operation value that does not fit the expected type (rule V1).
type ValueError struct {
	Op       int
	Path     string
	Expected string // canonical type text
	Got      string // description of the given value
	Detail   string
}

func (e *ValueError) Error() string {
	d := "expected " + e.Expected + ", got " + e.Got
	if e.Detail != "" {
		d += ": " + e.Detail
	}
	return errText(e.Op, e.Path, ErrBadValue, d)
}
func (e *ValueError) Unwrap() error { return ErrBadValue }

// NotEditableError reports an edit of a value that has no editable source (API.md §7.2).
type NotEditableError struct {
	Op     int
	Path   string
	Reason Reason
	Origin string // ReasonComputed: canonical path of the nearest editable source, or ""
	Layer  string // ReasonLayered
	Detail string
}

func (e *NotEditableError) Error() string {
	d := string(e.Reason)
	if e.Detail != "" {
		d += ": " + e.Detail
	}
	return errText(e.Op, e.Path, ErrNotEditable, d)
}
func (e *NotEditableError) Unwrap() error { return ErrNotEditable }

// StaleError reports files that changed since the base revision (rules S5, N9).
type StaleError struct {
	Files []string
}

func (e *StaleError) Error() string {
	return errText(-1, "", ErrStale, fmt.Sprint(e.Files))
}
func (e *StaleError) Unwrap() error { return ErrStale }

// RejectedError reports an edit refused because the result has error findings (rule E19).
type RejectedError struct {
	Findings []Finding // the error findings, rule F2 order
}

func (e *RejectedError) Error() string {
	return errText(-1, "", ErrRejected, fmt.Sprintf("%d error(s)", len(e.Findings)))
}
func (e *RejectedError) Unwrap() error { return ErrRejected }

// NotCanonicalError lists files an edit would write that are not in canonical layout
// (rule M9).
type NotCanonicalError struct {
	Files []string
}

func (e *NotCanonicalError) Error() string {
	return errText(-1, "", ErrNotCanonical, fmt.Sprint(e.Files))
}
func (e *NotCanonicalError) Unwrap() error { return ErrNotCanonical }

// SyntaxError is returned by Format and FormatJSONSource for input that does not parse.
type SyntaxError struct {
	Findings []Finding
}

func (e *SyntaxError) Error() string {
	if len(e.Findings) > 0 {
		f := e.Findings[0]
		return fmt.Sprintf("%s:%d:%d: %s", f.File, f.Line, f.Col, f.Message)
	}
	return "syntax error"
}

// InternalError is a compiler bug recovered at the API boundary (rule X2).
type InternalError struct {
	Msg   string
	Stack string // Go stack trace
}

func (e *InternalError) Error() string { return ErrInternal.Error() + ": " + e.Msg }
func (e *InternalError) Unwrap() error { return ErrInternal }
