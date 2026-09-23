package syntax

// BadExpr stands for an expression, annotation value or project value that did not parse; its
// bounds are the tokens skipped or, when none was, empty (Last = First-1) at the unexpected one.
type BadExpr struct {
	Bounds
}

func (*BadExpr) Kind() NodeKind                { return KindBadExpr }
func (*BadExpr) exprNode()                     {}
func (*BadExpr) annValueNode()                 {}
func (*BadExpr) projectValueNode()             {}
func (*BadExpr) children(func(Node) bool) bool { return true }

// BadType stands for a type that did not parse, bounded like BadExpr.
type BadType struct {
	Bounds
}

func (*BadType) Kind() NodeKind                { return KindBadType }
func (*BadType) typeNode()                     {}
func (*BadType) children(func(Node) bool) bool { return true }

// BadStmt stands for a statement that did not parse, up to the separator the parser resumed at.
type BadStmt struct {
	Bounds
}

func (*BadStmt) Kind() NodeKind                { return KindBadStmt }
func (*BadStmt) stmtNode()                     {}
func (*BadStmt) children(func(Node) bool) bool { return true }

// BadDecl stands for a declaration or list item that did not parse, up to the separator the
// parser resumed at: an item of a file, a record, variant or view body, a group or a literal.
type BadDecl struct {
	Bounds
}

func (*BadDecl) Kind() NodeKind                { return KindBadDecl }
func (*BadDecl) declNode()                     {}
func (*BadDecl) recordItemNode()               {}
func (*BadDecl) variantItemNode()              {}
func (*BadDecl) viewItemNode()                 {}
func (*BadDecl) groupMemberNode()              {}
func (*BadDecl) braceItemNode()                {}
func (*BadDecl) children(func(Node) bool) bool { return true }
