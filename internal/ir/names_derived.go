package ir

// foreignNote is how E8005 names a declared name otherwise than as declared (CODEGEN.md §7.9, DECISIONS 442, 458): built on a Canon name that @<target>(name:) does not change (derived: a foreign define's foreignDerived, and a collision with another such name `derived`), fixed by the generator (foreignFixed), or at another origin, the record of <Rec>Id, <Rec>IdFromWire and <Element>Row.
type foreignNote struct {
	derived bool   // built on Canon names @<target>(name:) does not change, but for the owner's below
	fixed   bool   // no declaration's name or override changes it (foreignFixed)
	origin  string // the origin the finding names, "" for the declared one
	owner   any    // a derived name's owner (a class, a dependent type) whose own override renames it: two names meet as `derived` only under one owner, else that override separates them
}

// noteCtx is the name a noteDeclared is declaring, with its note.
type noteCtx struct {
	name string
	note foreignNote
}

// noteDeclared runs declare, which declares name in sc, and keeps note for name when that declaration added it: a name another item holds keeps that item's. A collision declare meets is E8005 `derived` (DECISIONS 442, 458) when both names are derived and share an owner, which record decides from the note of the name already held.
func (n *namer) noteDeclared(sc *nameScope, name string, note foreignNote, declare func()) {
	_, taken := sc.names[name]
	prev := n.cur
	n.cur = &noteCtx{name: name, note: note}
	declare()
	n.cur = prev
	if _, added := sc.names[name]; !added || taken {
		return
	}
	if sc.notes == nil {
		sc.notes = map[string]foreignNote{}
	}
	sc.notes[name] = note
}

// record keeps a collision of pair: the first of a pair of origins is the problem, and a later one that is `derived` stays beside it as Alt, which reports the pair when nothing above `derived` (reached, finder, idType) does (DECISIONS 203, 458: a Canon rename fixes a pair whole, an override only one name). held is the note of the name the first origin declared.
func (n *namer) record(pair originPair, pr GoNameProblem, held foreignNote) {
	if c := n.cur; c != nil && c.name == pr.Name && c.note.derived && held.derived && c.note.owner == held.owner {
		pr.Derived = true
	}
	at, seen := n.reported[pair]
	if !seen {
		n.reported[pair] = len(n.problems)
		n.problems = append(n.problems, pr)
		return
	}
	if first := &n.problems[at]; pr.Derived && !first.Derived && first.Alt == nil {
		first.Alt = &pr
	}
}

// heldNote is the note of name as sc holds it for first, the origin that declared it, or as the outer scope does when sc only hides it (a class scope hides its own name, which the namespace declared); names declared apart from noteDeclared have none.
func (n *namer) heldNote(sc *nameScope, name, first string) foreignNote {
	switch {
	case sc.names[name] == first:
		return sc.notes[name]
	case sc.hidden[name] == first && n.outer != nil && n.outer.names[name] == first:
		return n.outer.notes[name]
	}
	return foreignNote{}
}
