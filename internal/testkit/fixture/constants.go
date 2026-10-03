package fixture

// The directory the copy leaves behind and the modes it writes with.
const (
	skippedDir = "expected"
	dirMode    = 0o750
	fileMode   = 0o600
)

// renamesProject is the project file of a temporary copy of examples/features/renames: its
// packages, the studio vocabulary its view imports and what that imports, every root inside.
const renamesProject = `project renames {
  canon: "0.1"

  roots {
    source: "out/source"
    generated: "out/generated"
  }

  languages: [en, fr]
  studio: studio
}
`

// renamesSources are the example trees the copy takes, by directory under examples/.
var renamesSources = []string{"features/renames", "studio", "sovcommon/time"}
