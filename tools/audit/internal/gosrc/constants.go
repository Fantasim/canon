package gosrc

import "golang.org/x/tools/go/packages"

// Names more than one lane resolves Go code against.
const (
	TestPkgSuffix = "_test"
	PkgNetHTTP    = "net/http"
	MainPkg       = "main"
)

const (
	testSuffix = "_test.go"
	genSuffix  = ".gen.go"
	e2eDirName = "e2e"
	stubSuffix = "stub"
	pathSep    = "/"
	typedMode  = packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes |
		packages.NeedTypesInfo | packages.NeedImports | packages.NeedModule
)
