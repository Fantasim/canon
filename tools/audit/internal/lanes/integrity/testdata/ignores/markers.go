package markers

// sovaudit:ignore magic-number -- a fixture
const two = 2

func g() error { return nil }

func f() {
	_ = "//nolint and #nosec in a string are not ignores"
	g() //nolint:errcheck // fixture
	g() //lint:ignore SA1000 fixture
	//lint:file-ignore SA1000 fixture
	g() // #nosec G101 -- fixture
	g() //gosec:disable G101 -- fixture
	//exhaustive:ignore
	//revive:disable-next-line:var-naming
	_ = two
}
