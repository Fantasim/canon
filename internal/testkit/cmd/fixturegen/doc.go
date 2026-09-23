// Command fixturegen regenerates the committed fixtures of examples/_fixtures from the real
// data copied into the git-ignored testdata-real/, following the embedded list fixtures.tsv:
// fixed files, fixed names, fixed order, so the same real data always gives the same bytes.
// Without testdata-real/ it fails and writes nothing. Run it only on purpose, from the
// repository root, and review the diff:
//
//	go run ./internal/testkit/cmd/fixturegen [-real testdata-real] [-fixtures examples/_fixtures]
package main
