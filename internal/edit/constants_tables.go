package edit

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
)

// expanders turn a Change into the files a commit writes or removes, by kind (API.md §8.1, N8).
var expanders = [...]func(Change) []commitFile{
	ChangeModified:   modifiedFiles,
	ChangeCreated:    createdFiles,
	ChangeDeleted:    deletedFiles,
	ChangeRenamed:    renamedFiles,
	ChangeRemovedDir: removedDirFiles,
}

// steppers moves a walk one step from each state (API.md W3).
var steppers = [...]func(*judge, cursor, int) cursor{
	stTree:     treeStep,
	stAbsent:   absentStep,
	stSpread:   leave,
	stComputed: stay,
	stFormat:   stay,
	stLayered:  stay,
	stOpaque:   leaveOpaque,
	stLoaded:   stay,
}

// rows are API.md §7.2's rows in table order: the first that applies names the reason.
var rows = [...]func(*judge) Reason{computedRow, layeredRow, shiftRow, spreadRow, blindRow, exprRow, formatRow, starRow, inputRow, keyRow, pseudoRow, orderRow, lineRow, layerRow, undoRow}

// handlers apply each operation of API.md §8.3 to the current state.
var handlers = [...]func(*opCtx) error{
	OpSet: setOp, OpReset: resetOp, OpAdd: addOp, OpInsert: insertOp, OpAddEntry: addEntryOp,
	OpRemove: removeOp, OpMove: moveOp, OpRename: renameOp, OpRetire: retireOp,
	OpUnretire: unretireOp, OpSetCase: setCaseOp,
}

// fitters say, per operation, whether its target takes it without an edit layer: what handlers
// check before ErrBadOp or ErrStableKey, the values aside (API.md 8.3, E4).
var fitters = [...]func(*Snapshot, resolution) bool{
	OpSet: fitsAny, OpReset: fitsReset, OpAdd: fitsList, OpInsert: fitsList, OpAddEntry: fitsEntries,
	OpRemove: fitsRemove, OpMove: fitsItem, OpRename: fitsRename, OpRetire: fitsRetire,
	OpUnretire: fitsNone, OpSetCase: fitsCase,
}

// literalOps are DECISIONS 462's allowlist: the ops a literal in place of a load or an expression
// lets pass, each with what it still needs (literalPasses); no other op gets that way out.
var literalOps = [...]func(*judge) bool{
	OpSet: literalSet, OpReset: literalReset, OpAddEntry: literalAddEntry, OpRemove: literalRemove,
}

// jsonUnparsed are the failures that make a JSON source a file that does not parse (M9).
var jsonUnparsed = [...]error{jsonsrc.ErrSyntax, jsonsrc.ErrDuplicateKey, jsonsrc.ErrEncoding}

// scopeKinds are the declarations a bare name in scope names a value of, never a key (DECISIONS 318).
var scopeKinds = map[check.ObjKind]bool{
	check.ObjConst: true, check.ObjLet: true, check.ObjParam: true, check.ObjLocal: true, check.ObjField: true, check.ObjFn: true,
}

// noValueCodes leave the value they lie in none: EVALUATION.md §7.1's hard decode and typing codes.
var noValueCodes = codeSet(
	diag.E2102, diag.E2110, diag.E3002, diag.E3003, diag.E3008, diag.E3301, diag.E3312,
	diag.E7001, diag.E7002, diag.E7003, diag.E7004, diag.E7005, diag.E7006, diag.E7007, diag.E7008,
	diag.E7009, diag.E7102, diag.E7103, diag.E7104, diag.E7105, diag.E7106, diag.E7108, diag.E7109,
	diag.E7110, diag.E7111, diag.E7112, diag.E7113, diag.E7114, diag.E7116, diag.E7117,
)
