package build

// listingChanges are a canon.outputs' writes for a killed build to stay recoverable: first, the new
// list plus the previous one's other lines and hashes, so all it may rename in is owned and all
// it may not have removed stays removable; last, after the removals, the new list alone.
func (r *run) listingChanges(o *output) (change, *change) {
	c := change{abs: o.Abs, display: o.Path, data: o.Content, old: o.old, existed: o.existed}
	if !o.existed || !namesDir(o.Package)(o.old) {
		return c, nil
	}
	entries := listEntries(o.Content)
	listed := map[string]bool{}
	for _, e := range entries {
		listed[e.path] = true
	}
	all := entries
	for _, e := range listEntries(o.old) {
		if !listed[e.path] {
			all = append(all, e)
		}
	}
	if len(all) == len(entries) {
		return c, nil
	}
	dir, _ := r.packageDir(o.Package) // found: o is an output of the run's IR
	c.data = listingOf(dir, all)
	return c, &change{abs: o.Abs, display: o.Path, data: o.Content, old: c.data, existed: true, late: true}
}

// ordered are the changes of a write in the order that keeps a killed build recoverable: the
// lists of text files first, the directory renames, then every other output and lock, the
// removals, and the lists' final forms last (DECISIONS 326, 336, 351, 377).
func (r *run) ordered(out *BuildResult, outputs []*output, locks []*lockOut) []change {
	var first, changes, removals, last []change
	for _, o := range outputs {
		written := o.Status == StatusWritten || o.Status == StatusAdopted
		switch {
		case o.remove:
			removals = append(removals, change{abs: o.Abs, display: o.Path, remove: true})
		case written && o.listing:
			c, final := r.listingChanges(o)
			first = append(first, c)
			if final != nil {
				last = append(last, *final)
			}
		case written:
			changes = append(changes, change{abs: o.Abs, display: o.Path, data: o.Content, old: o.old, existed: o.existed, respell: o.respell})
		}
		out.Outputs = append(out.Outputs, o.Output)
	}
	for _, l := range locks {
		if l.Status == StatusWritten {
			changes = append(changes, change{abs: l.Abs, display: l.Path, data: l.Content, old: l.old, existed: l.old != nil})
		}
		out.Locks = append(out.Locks, l.Lock)
	}
	return append(append(append(append(first, r.dirChanges()...), changes...), removals...), last...)
}

// dirChanges are the directory renames, top-down, each before any output below it (DECISIONS 377).
func (r *run) dirChanges() []change {
	out := make([]change, len(r.renames))
	for i, rn := range r.renames {
		out[i] = change{abs: rn.to, display: rn.display, respell: rn.from, dir: true}
	}
	return out
}
