package fnsize

import (
	"fmt"
	"go/ast"
	"go/parser"
	gotoken "go/token"
)

// Func is one function found in a source: its name, the lines strictly between its opening and
// closing brace, and the statements at the top level of its body.
type Func struct {
	Name       string
	Body       int
	Statements int
}

// Go measures the functions and methods of a Go file, and the function literals its package-level
// variables hold; a literal inside a function belongs to that function. A method is named
// Receiver.Name.
func Go(src string) ([]Func, error) {
	fset := gotoken.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errParse, err)
	}
	var funcs []Func
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Body != nil {
				funcs = append(funcs, goFunc(fset, funcName(d), d.Body))
			}
		case *ast.GenDecl:
			funcs = append(funcs, varLiterals(fset, d)...)
		}
	}
	return funcs, nil
}

// Cpp measures the function bodies of a C++ file: functions, members defined in a class body,
// constructors, operators, and the outermost lambda of an initializer, named after its variable.
// A header it cannot classify is an error, never a skipped function.
func Cpp(src string) ([]Func, error) {
	toks, match, err := tokenize(src, cppLanguage)
	if err != nil {
		return nil, err
	}
	return scanCpp(toks, match)
}

// TS measures the function bodies of a TypeScript file: functions, methods, constructors and
// accessors, and arrows with a block body. An arrow whose body is an expression has no braces to
// measure; one nested in a function belongs to it.
func TS(src string) ([]Func, error) {
	toks, match, err := tokenize(src, tsLanguage)
	if err != nil {
		return nil, err
	}
	return scanTS(toks, match), nil
}

// tokenize lexes src and pairs its brackets.
func tokenize(src string, lang *language) ([]token, []int, error) {
	toks, err := lex(src, lang)
	if err != nil {
		return nil, nil, err
	}
	match, err := matchBrackets(toks)
	return toks, match, err
}

// newFunc measures the body whose opening brace is toks[open].
func newFunc(toks []token, match []int, name string, open int) Func {
	end := match[open]
	return Func{
		Name:       name,
		Body:       max(0, toks[end].line-toks[open].line-1),
		Statements: countStatements(toks, match, open+1, end),
	}
}

func isPunct(t token, text string) bool { return t.kind == kindPunct && t.text == text }

func isWord(t token, text string) bool { return t.kind == kindIdent && t.text == text }
