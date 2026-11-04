package edit

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
