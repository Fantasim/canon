// Package project reports E1001 and E1002: a comment may cite a code.
package project

import (
	"errors"
	"fmt"

	"example.com/canon/internal/diag"
)

var errLoose = errors.New("doc comment is not attached to anything")

func Check(bag *diag.Bag, n int, key string) {
	diag.E1001.At(1, "1.0", nil).Check("clean_name").Report(bag)
	diag.E1002.At(1, fmt.Sprintf("%d", n)).Report(bag)
	diag.E1001.At(2, key, []string{"0.1"}).Check("a phrase here").Report(bag)
	b := diag.E1002.At(3, key)
	b.Check(fmt.Sprint(key)).Report(bag)
	_ = "E1002"
	_ = "E" + "1002"
	_ = "a code W10011 is no code"
	f := diag.Finding{Message: key}
	f.Message = key
	_, _ = f, errLoose
}
