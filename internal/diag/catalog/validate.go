package catalog

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
)

// validate applies the refusals of ERRORS.md §2.1 that concern the catalogue as a whole.
func validate(c *Catalog, pkgs map[string]bool, secs []section) error {
	errs := []error{validateArgTypes(c.ArgTypes)}
	types := map[string]bool{}
	for _, t := range c.ArgTypes {
		types[t.Name] = true
	}
	seen := map[string]bool{}
	for _, code := range c.Codes {
		errs = append(errs, validateCode(code, pkgs, seen), validateVariants(code))
		for _, m := range code.Messages {
			errs = append(errs, validateMessage(m, types))
		}
	}
	errs = append(errs, validateKinds(c), validateRuntime(c), validateCount(c, secs))
	return errors.Join(errs...)
}

func validateArgTypes(types []ArgType) error {
	seen := map[string]bool{}
	for _, t := range types {
		if !reUpperCamel.MatchString(t.Name) || seen[t.Name] || t.GoType == "" {
			return fmt.Errorf("%w: argument type %q is invalid, repeated or has no Go type", errArgs, t.Name)
		}
		seen[t.Name] = true
	}
	return nil
}

func validateCode(c Code, pkgs, seen map[string]bool) error {
	at := fmt.Sprintf(fmtAt, c.line, c.ID)
	switch {
	case !reCode.MatchString(c.ID):
		return fmt.Errorf("%w: %s: not a code", errCode, at)
	case seen[c.ID]:
		return fmt.Errorf("%w: %s: listed twice", errCode, at)
	case slices.Contains(retiredCodes, c.ID):
		return fmt.Errorf("%w: %s: a retired number", errCode, at)
	case !slices.Contains(severities, c.Severity):
		return fmt.Errorf("%w: %s: severity %q", errSeverity, at, c.Severity)
	case (c.Severity == severityWarning) != (c.ID[0] == warningLetter):
		return fmt.Errorf("%w: %s: the letter disagrees with severity %s", errSeverity, at, c.Severity)
	case (c.Severity == severityRuntime) != (c.Package == genPackage):
		return fmt.Errorf("%w: %s: package %s is for runtime codes only, and runtime codes belong to it", errPackage, at, genPackage)
	case c.Package != genPackage && !pkgs[c.Package]:
		return fmt.Errorf("%w: %s: package %q is not in the package table", errPackage, at, c.Package)
	case len(c.Messages) == 0:
		return fmt.Errorf("%w: %s: no message row", errMessage, at)
	}
	seen[c.ID] = true
	return nil
}

// validateVariants: one message has no variant name; several have distinct lowerCamel names.
func validateVariants(c Code) error {
	names := map[string]bool{}
	for _, m := range c.Messages {
		named := m.Variant != ""
		if named != (len(c.Messages) > 1) || (named && (!reLowerCamel.MatchString(m.Variant) || names[m.Variant])) {
			return fmt.Errorf("%w: line %d: variant %q of %s", errMessage, m.line, m.Variant, c.ID)
		}
		names[m.Variant] = true
	}
	return nil
}

func validateMessage(m Message, types map[string]bool) error {
	if len(m.Args) > maxArgs {
		return fmt.Errorf("%w: line %d: %d arguments, at most %d", errArgs, m.line, len(m.Args), maxArgs)
	}
	declared := map[string]bool{}
	for _, a := range m.Args {
		if !reLowerCamel.MatchString(a.Name) || declared[a.Name] || slices.Contains(reservedNames, a.Name) || !types[a.Type] {
			return fmt.Errorf("%w: line %d: argument %s:%s", errArgs, m.line, a.Name, a.Type)
		}
		declared[a.Name] = true
	}
	used, err := placeholders(m.Template)
	if err != nil {
		return fmt.Errorf("line %d: %w", m.line, err)
	}
	for _, name := range used {
		if !declared[name] {
			return fmt.Errorf("%w: line %d: placeholder {%s} names no argument", errTemplate, m.line, name)
		}
	}
	for _, a := range m.Args {
		if !slices.Contains(used, a.Name) {
			return fmt.Errorf("%w: line %d: argument %s is not used", errTemplate, m.line, a.Name)
		}
	}
	return nil
}

// validateRuntime: every pair of ERRORS.md §1.6 names a code and is listed once.
func validateRuntime(c *Catalog) error {
	codes := codeSet(c)
	seen := map[RuntimeText]bool{}
	for _, r := range c.Runtime {
		if !codes[r.Code] || r.Text == "" || seen[r] {
			return fmt.Errorf("%w: pair (%s, %q) names no code, is empty or is listed twice", errRuntime, r.Code, r.Text)
		}
		seen[r] = true
	}
	return nil
}

// validateCount checks the one count sentence against the tables.
func validateCount(c *Catalog, secs []section) error {
	var found [][]string
	for _, s := range secs {
		for _, l := range s.lines {
			if m := reCount.FindStringSubmatch(l.text); m != nil {
				found = append(found, m[1:])
			}
		}
	}
	if len(found) != 1 {
		return fmt.Errorf("%w: %d count sentences, want exactly one", errCount, len(found))
	}
	want := countFields(c)
	for i, got := range found[0] {
		if got != strconv.Itoa(want[i]) {
			return fmt.Errorf("%w: the sentence says %v, the tables hold %v", errCount, found[0], want)
		}
	}
	return nil
}

// countFields is what the count sentence states: codes, errors, warnings, runtime codes and
// messages.
func countFields(c *Catalog) []int {
	bySeverity := map[string]int{}
	messages := 0
	for _, code := range c.Codes {
		bySeverity[code.Severity]++
		messages += len(code.Messages)
	}
	return []int{len(c.Codes), bySeverity[severityError], bySeverity[severityWarning], bySeverity[severityRuntime], messages}
}

func codeSet(c *Catalog) map[string]bool {
	out := map[string]bool{}
	for _, code := range c.Codes {
		out[code.ID] = true
	}
	return out
}
