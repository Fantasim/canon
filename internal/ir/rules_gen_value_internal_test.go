package ir

import (
	"testing"

	"github.com/fantasim/canonlang/internal/value"
)

// CODEGEN.md §5.1, §5.13: a table default with a row holds a record value (E8019 RecordConstant for gen/cpp), an empty one does not.
func TestHoldsRecordValueTable(t *testing.T) {
	if !holdsRecordValue(&value.Table{Entries: []*value.Record{{}}}) {
		t.Error("a table with a row holds no record value")
	}
	if holdsRecordValue(&value.Table{}) {
		t.Error("an empty table holds a record value")
	}
}
