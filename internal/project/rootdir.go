package project

// RootDir is root's directory as the layout places it, never resolved through links; "" for an undeclared root.
func (l *Layout) RootDir(root string) string {
	return l.dirs[root]
}
