# tools/tsc

The TypeScript toolchain the generated-code tests use (CODEGEN.md §9, IMPLEMENTATION-PLAN.md §7.8,
DECISIONS 277). It is for tests and CI only: nothing in the compiler or its output depends on it.

- `typescript5` is TypeScript 5.0, the floor; `typescript-latest` is the current release; `@types/node`
  types the conformance tests (`node:test`, `node:assert`).
- Install: `npm ci --prefix tools/tsc` (network is needed for this step only).
- `internal/gen/ts` runs `tsc --strict --noEmit --noUnusedLocals` with both compilers over every
  generated file and `node --test` over the compiled conformance tests. Without the install the tests
  skip; with `CANON_REQUIRE_TS=1` (set by `make check` and CI) they fail instead.

Refreshing the current compiler: `npm view typescript version` names the release; set `typescript-latest`
in `package.json` to `npm:typescript@<version>`, run `npm install --prefix tools/tsc`, run the tests and
commit `package.json` and `package-lock.json`. `typescript5` stays on 5.0.x, the floor of CODEGEN.md §9.
