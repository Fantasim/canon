package gorules

import (
	"regexp"

	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
)

const laneName = "gorules"

const (
	ruleMagicString   = "magic-string"
	ruleMagicNumber   = "magic-number"
	ruleConstPlace    = "const-placement"
	ruleConstDup      = "const-dup"
	ruleEnvKey        = "env-key"
	ruleErrInline     = "err-inline"
	ruleErrPlace      = "err-placement"
	ruleExportedLocal = "exported-but-local"
	ruleLogDirect     = "log-direct"
	ruleBareGo        = "bare-goroutine"
	ruleImportBound   = "import-boundary"
	ruleStdlib        = "reimplements-stdlib"
)

var ruleIDs = []string{
	ruleMagicString,
	ruleMagicNumber,
	ruleConstPlace,
	ruleConstDup,
	ruleEnvKey,
	ruleErrInline,
	ruleErrPlace,
	ruleExportedLocal,
	ruleLogDirect,
	ruleBareGo,
	ruleImportBound,
	ruleStdlib,
}

// File and directory names the rules place things in.
const (
	constantsFile  = "constants.go"
	errorsFile     = "errors.go"
	configDirName  = "config"
	cmdDirName     = "cmd"
	mainPkg        = gosrc.MainPkg
	safegoSuffix   = "/safego"
	pathSep        = "/"
	boundaryAny    = "/..."
	commentMark    = "#"
	fieldSep       = "\t"
	underscore     = "_"
	dirHere        = "."
	testPkgSuffix  = gosrc.TestPkgSuffix
	testMainSuffix = ".test"
)

// Import paths and names the rules resolve calls against.
const (
	pkgOS      = "os"
	pkgLog     = "log"
	pkgFmt     = gosrc.PkgFmt
	pkgErrors  = "errors"
	pkgHTTP    = gosrc.PkgNetHTTP
	pkgRegexp  = "regexp"
	pkgSort    = "sort"
	pkgSlices  = "slices"
	pkgStrconv = "strconv"
	pkgTime    = "time"
	pkgSlog    = "log/slog"

	fnNew           = "New"
	fnErrorf        = "Errorf"
	fnMustCompile   = "MustCompile"
	fnMustCompilePX = "MustCompilePOSIX"
	fnAppend        = "append"
	fnMake          = "make"
	sortSlice       = "Slice"
	sortSliceStable = "SliceStable"
	fnLen           = "len"
	identTrue       = "true"
	identFalse      = "false"
	identString     = "string"
	wrapVerb        = "%w"
	funcLitName     = "func literal"
	fnMethod        = "Method"
	typeDuration    = "Duration"
	logMark         = "log"
)

var (
	envFuncs     = []string{"Getenv", "LookupEnv", "Setenv", "Unsetenv", "ExpandEnv"}
	logPrefixes  = []string{"Print", "Fatal", "Panic"}
	fmtPrints    = []string{"Print", "Printf", "Println"}
	builtinPrint = []string{"print", "println"}
	naturalSorts = map[string][]string{pkgSort: {"Strings", "Ints", "Float64s"}, pkgSlices: {"Sort"}}
	logPkgs      = []string{pkgLog, pkgSlog}
	plainNumbers = []int64{0, 1}
	timeUnits    = []string{"Nanosecond", "Microsecond", "Millisecond", "Second", "Minute", "Hour"}
)

// loggerMethods are slog.Logger's logging methods, trusted by name when types are missing.
var loggerMethods = []string{
	"Debug", "Info", "Warn", "Error", "DebugContext", "InfoContext", "WarnContext", "ErrorContext",
	"Log", "LogAttrs", "With", "WithGroup",
}

// routeFuncs register a route; routerTypeMarks name the types they are methods of.
var (
	routeFuncs      = []string{"Get", "Post", "Put", "Patch", "Delete", "Head", "Options", "Handle", "HandleFunc", "Method"}
	routerTypeMarks = []string{"Router", "Mux"}
)

// stdLayouts are time layouts: the value alone says what it is.
var stdLayouts = map[string]string{
	"2006-01-02":                          "time.DateOnly",
	"15:04:05":                            "time.TimeOnly",
	"2006-01-02 15:04:05":                 "time.DateTime",
	"2006-01-02T15:04:05Z07:00":           "time.RFC3339",
	"2006-01-02T15:04:05.999999999Z07:00": "time.RFC3339Nano",
}

// httpMethods are only suggested where the file imports net/http ("HEAD" is also a git ref).
var httpMethods = map[string]string{
	"GET":     "http.MethodGet",
	"POST":    "http.MethodPost",
	"PUT":     "http.MethodPut",
	"PATCH":   "http.MethodPatch",
	"DELETE":  "http.MethodDelete",
	"HEAD":    "http.MethodHead",
	"OPTIONS": "http.MethodOptions",
}

// initialisms are lowered whole when unexporting a name that starts with one.
var initialisms = []string{
	"ACL", "API", "ASCII", "CPU", "CSS", "CSV", "DB", "DNS", "EOF", "GUID", "HTML", "HTTP", "HTTPS", "ID",
	"IP", "JSON", "JWT", "OK", "RAM", "RPC", "SQL", "SSH", "TCP", "TLS", "TOTP", "TTL", "UDP", "UI", "UID",
	"URI", "URL", "UTF8", "UUID", "XML",
}

// shape is what a ranged-over expression is known to be from its declaration.
type shape int

const (
	shapeUnknown shape = iota
	shapeSlice
	shapeMap
	shapeString
)

// wellKnownMethods are satisfied by reflection or by interfaces outside any load.
var wellKnownMethods = []string{
	"Error", "String", "ServeHTTP", "MarshalJSON", "UnmarshalJSON", "MarshalText", "UnmarshalText",
	"Scan", "Value", "Unwrap", "Is", "As", "Format", "GoString", "LogValue", "Handle", "Enabled",
	"WithAttrs", "WithGroup", "Len", "Less", "Swap", "Read", "Write", "Close",
}

// wordish is a single identifier-like token: "status", "created_at", "Icon", "not-configured".
var wordish = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// sqlFragment is a value made only of upper-case SQL keywords: " AND ", "ORDER BY", "LIMIT".
var sqlFragment = regexp.MustCompile(`^[\s,()]*(` + sqlKeywords + `)([\s,()]+(` + sqlKeywords + `))*[\s,()]*$`)

const sqlKeywords = `SELECT|FROM|WHERE|AND|OR|NOT|IN|IS|NULL|LIMIT|OFFSET|ORDER|GROUP|BY|HAVING|ASC|DESC|` +
	`JOIN|LEFT|RIGHT|INNER|OUTER|ON|AS|SET|VALUES|INSERT|INTO|UPDATE|DELETE|RETURNING|DISTINCT|LIKE|` +
	`ILIKE|BETWEEN|UNION|ALL|CASE|WHEN|THEN|ELSE|END|EXISTS|NULLS|FIRST|LAST|CONFLICT|DO|NOTHING|WITH`

// sqlLead is the blanks and SQL comments ahead of a statement's first word.
var sqlLead = regexp.MustCompile(`^(\s|--[^\n]*(\n|$)|/\*(?s:.*?)\*/)*`)

// sqlVerb is a statement's first word: a verb that starts a whole SQL statement, followed by
// a blank or "(". Clause words (FROM, WHERE, ORDER, JOIN, VALUES, RETURNING) start fragments.
var sqlVerb = regexp.MustCompile(`^(?i:(select|with|insert|update|delete|merge|call|set|lock|truncate))[\s(]`)

// sqlOperand stands in for an operand of a SQL concatenation that cannot be folded.
const sqlOperand = "?"

// fmtVerb matches one fmt verb with its flags, width and precision.
var fmtVerb = regexp.MustCompile(`%[-+# 0]*(\[\d+\])?(\*|\d+)?(\.(\*|\d+)?)?[a-zA-Z%]`)

const (
	detailMax  = 120
	snippetMax = 60
	maxWordLen = 24
	singleRune = 1
	firstArg   = 0
	secondArg  = 1
	thirdArg   = 2
	pairLen    = 2
)

// Messages and fixes.
const (
	msgMagicString  = "%q x%d here (first %s:%d)"
	msgMagicShared  = "%q x%d here, x%d in %d packages (first %s:%d)"
	msgMagicNumber  = "%s in %s"
	fixConstIn      = "name it in %s"
	fixConstShared  = "name it once in the lowest package both import"
	fixUse          = "use %s"
	msgConstPlace   = "%s block of %d outside %s"
	fixMoveTo       = "move it to %s"
	msgConstDup     = "%q: %s duplicates %s (%s:%d)"
	fixConstDup     = "keep one: %s in %s"
	msgEnvOutside   = "os.%s outside the config package"
	msgEnvLiteral   = "os.%s with a literal key %q"
	fixEnvOutside   = "read it once in cmd/ (or a config package) and pass it down, key as a constant"
	fixEnvLiteral   = "name the key in %s"
	msgErrNew       = "errors.New(%s) inside a function"
	msgErrorf       = "fmt.Errorf(%s) without %%w"
	fixErrNew       = "declare a sentinel in %s and wrap it"
	fixErrorf       = "wrap a sentinel or the cause with %w"
	msgErrPlace     = "sentinel %s outside %s"
	msgExported     = "exported %s %s is used only inside its package"
	fixUnexportName = "unexport it (rename to %s)"
	msgLogDirect    = "%s outside cmd/ and package main"
	fixLogDirect    = "return it to the caller; only cmd/ prints"
	msgBareGo       = "go %s"
	fixBareGo       = "recover inside the goroutine (one helper package named safego) and return the panic as an error"
	msgBoundary     = "%s must not import %s (%s:%d)"
	fixBoundary     = "go through the allowed layer"
	msgStdlib       = "%s re-implements %s"
	skipTyped       = "type-checked load failed: %v"
	skipBoundaryRow = "%s:%d: want <from-dir>\\t<forbidden-import>, valid path.Match patterns"
	qualified       = "%s.%s"
	kindMethod      = "method"
	stdContains     = "slices.Contains"
	stdContainsRune = "strings.ContainsRune"
	stdIndex        = "slices.Index"
	stdIndexRune    = "strings.IndexRune"
	stdKeys         = "slices.Collect(maps.Keys(%s))"
	stdSortedKeys   = "slices.Sorted(maps.Keys(%s))"
	msgKeys         = "loop collecting the keys of %s re-implements %s"
	stdMin          = "the builtin min"
	stdMax          = "the builtin max"
	patContains     = "contains"
	patIndex        = "index"
	patKeys         = "map keys"
	patMin          = "min"
	patMax          = "max"
)
