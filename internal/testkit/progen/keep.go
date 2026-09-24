package progen

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/tools/txtar"
)

// Counterexample is a shrunk failing case kept as a txtar regression test: a "key value" header
// line per field, then the project's files. The tool writes every field but Open, which a
// person sets when triaging the bug.
type Counterexample struct {
	Suite    string   // the suite that found it
	Name     string   // what the suite ran: an operator's rule, a property
	Case     int      // the case's number when kept; a replay finds the operator by Name
	Seed     uint64   // the case's seed
	Open     string   // the package that owns the open bug; "" once it is fixed
	Sig      string   // the failure's signature: an open archive stands for this one only
	Packages []string // the selectors the case checks
	Layers   []string // the layers the case activates
	Want     string   // the suite's expectation, in its own words
	Note     string   // what failed, one line
	Files    *Project
}

// Format is the counterexample as a txtar archive.
func (c *Counterexample) Format() []byte {
	var head bytes.Buffer
	values := []string{
		c.Suite, c.Name, strconv.Itoa(c.Case), strconv.FormatUint(c.Seed, decimal), c.Open, c.Sig,
		strings.Join(c.Packages, fieldSep), strings.Join(c.Layers, fieldSep), c.Want, c.Note,
	}
	for i, key := range headerKeys {
		if values[i] != "" {
			fmt.Fprintf(&head, headerFormat, key, oneLine(values[i]))
		}
	}
	for _, name := range c.Files.linkNames() {
		fmt.Fprintf(&head, headerFormat, keyLink, name+fieldSep+c.Files.links[name])
	}
	a := &txtar.Archive{Comment: head.Bytes()}
	for _, name := range c.Files.Names() {
		data, _ := c.Files.Get(name)
		a.Files = append(a.Files, txtar.File{Name: name, Data: data})
	}
	return txtar.Format(a)
}

// ReadCounterexample parses a kept archive.
func ReadCounterexample(name string) (*Counterexample, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("progen: %w", err)
	}
	a := txtar.Parse(data)
	c := &Counterexample{Files: NewProject()}
	sc := bufio.NewScanner(bytes.NewReader(a.Comment))
	for sc.Scan() {
		key, value, _ := strings.Cut(sc.Text(), fieldSep)
		if err := c.set(key, value); err != nil {
			return nil, fmt.Errorf("%w: %s: %s", err, name, key)
		}
	}
	for _, f := range a.Files {
		c.Files.Set(f.Name, f.Data)
	}
	return c, nil
}

func (c *Counterexample) set(key, value string) error {
	fields := map[string]*string{keySuite: &c.Suite, keyName: &c.Name, keyOpen: &c.Open, keySig: &c.Sig, keyWant: &c.Want, keyNote: &c.Note}
	switch {
	case fields[key] != nil:
		*fields[key] = value
	case key == keyCase:
		n, err := strconv.Atoi(value)
		if err != nil {
			return errHeader
		}
		c.Case = n
	case key == keySeed:
		seed, err := strconv.ParseUint(value, decimal, seedBits)
		if err != nil {
			return errHeader
		}
		c.Seed = seed
	case key == keyPackages:
		c.Packages = strings.Fields(value)
	case key == keyLayers:
		c.Layers = strings.Fields(value)
	case key == keyLink:
		name, target, ok := strings.Cut(value, fieldSep)
		if !ok {
			return errHeader
		}
		c.Files.Link(name, target)
	default:
		return errHeader
	}
	return nil
}

// oneLine keeps a header value on its line.
func oneLine(s string) string { return strings.Join(strings.Fields(s), fieldSep) }
