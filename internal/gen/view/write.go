package viewgen

import (
	"reflect"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/jsonsrc"
)

// Write is the view model's exact bytes: a tree built from *m (VIEWMODEL.md J1-J3, J10), laid
// out by jsonsrc.Format, the one WIRE.md 7.4 printer (FORMATTER.md 14.1).
func Write(m *vm.ViewModel) ([]byte, error) {
	if m == nil {
		return nil, errNil
	}
	root, err := buildValue(reflect.ValueOf(*m))
	if err != nil {
		return nil, err
	}
	return jsonsrc.Format(root), nil
}
