package value

import (
	"strconv"

	"github.com/fantasim/canonlang/internal/types"
)

// Value is a Canon value. A value is immutable once it leaves the call that built it
// (EVL-04); the pointer is the instance, so copies share it (checks run once per instance).
type Value interface {
	Type() types.Type
	Prov() *Prov
	CanonText() string // canonical text form (STD-06); satisfies diag.ValueArg
}

// Key is an entry's or a ref's key: a string (an enum member's name) or an integer.
type Key struct {
	S     string
	I     int64
	IsInt bool
}

func (k Key) Text() string {
	if k.IsInt {
		return strconv.FormatInt(k.I, 10)
	}
	return k.S
}

// Identity is the collection instance and key of an entry (TYP-02); Owner is the record
// holding a collection that is a field.
type Identity struct {
	Coll    *types.Collection
	Owner   *Record
	Key     Key
	Retired bool
}

func (id *Identity) same(o *Identity) bool {
	return id.Coll == o.Coll && id.Owner == o.Owner && id.Key == o.Key
}
