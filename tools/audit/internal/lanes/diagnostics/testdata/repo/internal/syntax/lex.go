// Package syntax reports W1001 under a dot import.
package syntax

import . "example.com/canon/internal/diag"

func Lex(bag *Bag) { W1001.At(1).Report(bag) }
