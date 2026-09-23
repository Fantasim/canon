package catalog

import (
	"errors"
	"fmt"
	"slices"
)

// validateKinds: kinds list only codes taking a Kind, and each such code (ERRORS.md §1.4).
func validateKinds(c *Catalog) error {
	withKind := kindCodes(c)
	listed := map[string]bool{}
	names := map[string]bool{}
	var errs []error
	for _, k := range c.Kinds {
		if !reUpperCamel.MatchString(k.Name) || names[k.Name] || k.Word == "" || len(k.UsedBy) == 0 {
			errs = append(errs, fmt.Errorf("%w: line %d: kind %q is invalid, repeated, has no word or lists no code", errKind, k.line, k.Name))
		}
		names[k.Name] = true
		errs = append(errs, validateUsedBy(k, withKind, listed))
	}
	for _, code := range c.Codes {
		if withKind[code.ID] && !listed[code.ID] {
			errs = append(errs, fmt.Errorf("%w: %s takes a Kind argument and no kind lists it", errKind, code.ID))
		}
	}
	return errors.Join(errs...)
}

// validateUsedBy checks one kind's codes and records them in listed.
func validateUsedBy(k Kind, withKind, listed map[string]bool) error {
	for i, code := range k.UsedBy {
		if !withKind[code] || slices.Contains(k.UsedBy[:i], code) {
			return fmt.Errorf("%w: line %d: kind %s lists %s, which takes no Kind argument or is listed twice", errKind, k.line, k.Name, code)
		}
		listed[code] = true
	}
	return nil
}

// kindCodes is the set of codes with at least one Kind argument.
func kindCodes(c *Catalog) map[string]bool {
	out := map[string]bool{}
	for _, code := range c.Codes {
		if slices.ContainsFunc(code.Messages, takesKind) {
			out[code.ID] = true
		}
	}
	return out
}

func takesKind(m Message) bool {
	return slices.ContainsFunc(m.Args, func(a Arg) bool { return a.Type == kindType })
}

// firstKind is the first kind, in table order, that lists the code: the sample the
// generated constructor test gives a Kind argument.
func firstKind(c *Catalog, code string) string {
	for _, k := range c.Kinds {
		if slices.Contains(k.UsedBy, code) {
			return k.Name
		}
	}
	return ""
}
