# Decisions

Decisions taken while designing Canon. The spec follows them; changing one means changing
this file first.

## 2026-09-23 — Developer experience and philosophy

1. **The law lives in one central repository.** Every `.canon` file lives there, organized by
   domain. Data not yet converted (e.g. the 6,945 item files in Resource) stays where it is and
   is read by path until its domain moves into `.canon` (decision 11). `canon build` writes the generated Go, C++ and TypeScript into each consumer repo.

2. **Go, C++ and TypeScript receive generated code.** Other consumers (Lua bot scripts) get
   config through a program that already has it.

3. **Clean names in Canon, legacy names on the wire.** `heal: Int @json("nHeal")`. The studio
   shows "Heal", C++ gets `GetHeal()`, Go gets `Heal()`, data files keep `nHeal`. Renaming a
   Canon field therefore never rewrites data files.

4. **Generated types are read-only, always.** Every field is private and gets a getter; there
   is no setter and no opt-out. C++ getters follow the codebase convention (`GetXxx() const`);
   Go getters follow Go's (`Xxx()`); TypeScript uses `readonly` properties on frozen values.

5. **Legacy C++ structs: a per-struct toggle, from `const` to getters.** Each record chooses
   `@cpp(access: fields | both | getters)`:
   - `fields`: the hand-written struct stays; Canon fills it and lookups return `const`, so every
     existing read (`p->dwItemKind3`) compiles and every write is a compile error;
   - `both`: Canon generates the struct with the legacy public members **and** getters, so new code
     uses getters while old reads keep working;
   - `getters`: members are private; the compiler lists the reads left to migrate.

   Legacy systems start at `fields` and move to `getters` little by little, struct by struct.
   This is C++-only: Go cannot make a whole object read-only, so Go's hand-written config structs
   (e.g. sovcommon/teamboard) are replaced by generated getter types; TypeScript gets the same
   guarantee from `readonly`. Measured scale: `ItemProp*` appears 1,184 times (69 const today);
   `GetItemProp` is called 484 times.

6. **Canon is the law.** If data is wrong it is an error, even where a loader tolerates it
   (skips, clamps, falls back). Loaders can be simplified over time since they only receive
   checked data. Intentional exceptions are written as named `warn`s.

7. **Relevance comes from types, generated from the data.** An item's kind decides which fields
   exist (a material cannot carry attack speed). The first version of each kind is generated from
   the existing items by `canon infer`, permissive by design so the first build passes, then
   tightened in small reviewed steps. The data files do not change: the wire stays flat. The
   studio shows a kind's commonly used fields first and the rare ones under "More".

8. **Documentation is expected.** A public type or field without a `///` doc comment is a
   warning. Descriptions from the existing JSON Schemas and overlays are imported during
   migration, so most start documented.

9. **The studio serves developers and designers.** Designer-facing labels and help must make
   sense without knowing the C++; French translations are expected (missing ones are warnings).
   Technical fields go in `advanced` groups.

10. **Everything moves to Canon.** The end state: every config source is `.canon`, every loader
    is generated, validation exists only in Canon, and no runtime can modify config. The phases
    are an order of work, not options:
    1. Canon validates everything. `validate.py`, the JSON Schemas, the `x-` rules, the Lua
       rules and the studio overlays are deleted.
    2. Sources are converted to `.canon`, domain by domain (decision 11).
    3. Hand-written loaders are replaced by generated ones; legacy structs start in `fields` mode
       and move to `getters` struct by struct (decision 5).
    4. New systems use the generated classes directly.

11. **Data sources move into `.canon`, domain by domain.** The runtime never reads source files,
    only what `canon build` emits, so converting a source changes nothing for the server.
    `canon convert` turns loaded JSON into `.canon` source losslessly (one file per entry stays one
    file per entry), and the round-trip check proves it. Until a domain is converted, its JSON is
    read with `load`. Domains are converted one at a time, each with its round-trip proof, so a
    failure is always local to one domain.

12. **The studio edits values, never text.** It asks the compiler to set a value at a path, add
    or remove an entry; the compiler re-prints only what changed, in the canonical layout, and
    keeps comments. The same edit API works for `.canon` sources and for JSON sources, so
    `jsondoc`'s byte-preserving writes are no longer needed (JSON files are normalized to the
    canonical layout once). Views drive the screens, not the writing.

13. **No built-in data migrations.** Renaming a field never touches data (decision 3). Other
    changes (a wire name, a unit) are rare and codebase-specific: one-off scripts, reviewed like
    any change.

14. **Generated C++ is C++17 and never copies to read.** 57 of the 77 C++ projects build as
    C++17. Getters return scalars by value and everything else by `const&` (or `const T*` when
    optional); table lookups are binary searches over `std::string_view`, without allocation;
    data-mode tables load as a `std::shared_ptr<const …>` snapshot, so a reload is a pointer swap.

15. **Paths use named roots.** `project.canon` declares `@resource`, `@source`, `@services`…, so
    `load` and `emit` paths read `@resource/Server/...` instead of `../../../../..`.

16. **Reloadability is declared in the `.canon` file.** A value is loaded once unless marked
    `@reload`, which promises that runtime code keeps ids, not pointers, across a tick. Canon
    refuses `@reload` on data mapped to a legacy C++ struct in `fields` or `both` mode, because
    legacy code caches raw pointers to those (13 struct members do today). Reloadable values of
    one emit swap together as one immutable snapshot.

## 2026-09-23 — Triage of the implementability audit

The audit (AUDIT.md, 206 findings) and the studio mockup (meta/spec-phase/mockups/studio.html, 50 view gaps) were
triaged before writing the companion documents.

17. **Optionals are strict.** A `T?` is never used where a `T` is expected. Code proves presence:
    a `!= none` test narrows a local or an immutable path inside `if` (statements and
    expressions), `and`, `while` and comprehension `if` branches; `x ?? fallback` supplies a
    value; postfix `x!` asserts presence and is `E4001` at that expression if it is `none`.
    Examples are fixed accordingly. (Replaces AUDIT TYP-01's lenient default.)

18. **`canon fmt` never aligns columns.** One space after `:` and around `=`; no padding to line
    values up. A one-value edit must produce a one-line diff. Examples are reformatted.
    (Settles AUDIT FMT-01.)

19. **Every other proposed answer in AUDIT.md is accepted as written**, except:
    - **TYP-21 assets:** matching is exact and case-sensitive; a case-only match is the same error
      as a missing file (`E3701`). Legacy case drift in data is fixed by a one-off script, not by
      the language.
    - **CNF-02 runtime errors in translated code (C++):** no exceptions. Generated checked
      helpers call `canon::OnEvalError(code, message)`, a replaceable handler whose default logs
      and calls `std::abort()`. Go panics with `*rt.EvalError` (the generated `rt` helper package); TypeScript throws.

20. **The mockup's view gaps are closed with the agent's proposed sentences** (reported with the
    mockup), except gap 34 (asset letter case), which follows decision 19.

21. **Parallel legacy fields are real data.** `@json(pairs: ["dwDestParam{i}", "nAdjParamVal{i}"])`
    turns numbered wire keys into a list of records (`bonuses: [Bonus]`); code, checks and the
    studio all see the list. The view `row` alternative is dropped.

22. **A type can have a default editor.** `widget time_of_day(value: TimeOfDay) default` in the
    studio package applies to every field of that type; a view can still override it.

23. **The compiler lives at `github.com/fantasim/canonlang`** (Go module path, lower case). The
    language keeps the name Canon.

24. **Every other choice the documentation agents marked [confirm] is accepted**, as listed in
    `meta/spec-phase/review/ACCEPTED-CHOICES.md`. Where two agents disagreed, that file records which answer won.

25. **The compiler's own Go code follows the fleet's code doctrine** (`sovcommon/tools/sovaudit`,
    `fleet/DOCTRINE-code.md`):
    - functions ≤ 60 lines, ≤ 5 parameters, ≤ 3 results, nesting ≤ 3; files ≤ 500 lines;
    - no literal twice and no bare numbers but 0 and 1, with constants in `<pkg>/constants.go`;
    - sentinel errors in `<pkg>/errors.go`, wrapped with `%w`;
    - nothing exported that only its own package uses; no dead code, commented-out code or TODOs;
    - comments ≤ 3 lines that say why, with no file headers outside `doc.go`;
    - every package has a `doc.go` and an example test;
    - the ratchet baseline in `.sovaudit/`.

    sovaudit is **copied into the project** as `tools/audit` (its own Go module) and customised
    for a generic compiler repo:
    - fleet-only lanes are removed: comparisons against sovcommon and other services, Svelte and
      front-end rules, meta/ census;
    - "sovcommon first" becomes "standard library first";
    - generated goldens and testdata are excluded.

    `make check` = `gofmt -l` empty, `go vet`, `go test`, generated goldens diff-clean, and
    the auditor (`cd tools/audit && go run . check --repo ../..`, wired as `make audit-check`). Everything is in the public repo, so every contributor gets
    the same gate.

26. **The audit takes the strictest technically sound option; there are no exemptions.** All
    code is written by agents, and strict rules keep them from making mistakes.
    - Numbers: only 0 and 1 may appear bare.
    - A lexer or parser never grows past the size limits: dispatch over token kinds is
      table-driven (an array of small handler functions indexed by kind), and precedence and
      keyword tables are data in `constants.go`.
    - `// sovaudit:ignore` stays possible, with a reason, but the ignore count is ratcheted:
      it may never grow, and the target is zero.
    - Rule thresholds live in one data file, not in code, so a future UI can adjust them.

27. **Diagnostics are catalogue data, enforced end to end.**
    - `spec/ERRORS.md` is the single source; `internal/diag/codes.go` is generated from it and
      diff-checked by `make check`.
    - Code reports `diag.E3501.At(span, args…)`; no diagnostic message text appears anywhere else
      (audit rule).
    - Every catalogued code has at least one test that produces it (audit rule, ratcheted from
      M0).
    - Go sentinel errors in `errors.go` remain for Go-level failures only (I/O, stale revision,
      API errors), and the `err-*` rules apply to those.

28. **Local git, no remote.** `configlang/` is a git repository with local commits only (never
    a remote, never a push). Commits are small, happen after a green `make check`, and use
    `feat:`, `fix:`, `docs:`, `chore:` and similar prefixes.

29. **Real game data never enters git.** Copies of real Resource data used for tests live in
    `testdata-real/`, which is git-ignored and used by opt-in targets (`make check-real`).
    Committed fixtures stay small (`examples/_fixtures/`). Resource, Source and every other
    service are read-only from this project.

## Decided without Louis (autonomous session 2026-09-23 night), to review

Choices made while Louis was away are listed here, each with its reason, so he can overrule them.

30. **ERRORS.md holds every message, typed.** Each range has a codes table (with a `Package`
    column) and a messages table (`Code | Variant | Args | Template`); a code with several
    wordings has named variants, arguments are `name:Type` from a closed set of 15 types
    (`Name`, `Type`, `Value`, `Expr`, `Loc`, `Kind`…), at most 4 per message, closed word sets are
    a generated `diag.Kind` enum, and each variant is a typed constructor `diag.E2103.AtNone(span,
    …)`. Owning documents keep only the trigger. Reason: one source a generator can check and
    compile, with no English left to callers (DECISIONS 27).

31. **Rendered messages are plain text.** No Markdown quoting in templates: names print bare,
    values in their canonical text (strings quoted), so `E3501` reads `unknown key
    "II_SYS_SYS_SCR_FARM3" in resource.vocab.items`; the samples in SPEC, CLI and API were
    updated. Reason: the goldens already had no backticks, and one rule beats per-code habits.

32. **The fixed texts that generated code signals are catalogued too** (ERRORS.md §1.6), and
    `diaggen -runtime` checks the runtime helper texts against them. Reason: DECISIONS 27 allows
    message text nowhere else, and those helpers held 16 unlisted texts.

33. **`W1640` fires only for selected packages with an `emit view`**, like `W1701`; no golden
    changes. Reason: a package without a view model is not a studio concern, and M1's teamboard
    golden stays exact.

34. **Two injection seams, no more.** `check.Folder` (eval folds constants during checking,
    TYPES §15) and `eval.Host` (build supplies load and verify); `value` moves above `check` in
    the package table, `infer`/`convert` above `cli`. Reason: TYPES requires both calls and the
    dependency rule forbids the imports; injection keeps one evaluator.

35. **`?.` warns on a non-optional receiver segment, and a chain ending on an optional is not
    wrapped twice** (`w?.item.id`, never `w?.item?.id`). Reason: the report's answer; it gives
    every example one reading.

36. **A `ref` into a keyed list is not a finite parameter**: such a function is translated, and
    its ref parameter is `E9006`. Reason: stricter than adding keyed lists to lookup domains,
    which would need a key-to-ordinal map at run time that tables (id enums) do not.

37. **Emit rules are checked by `canon check`** (stage E builds and validates every emit's IR;
    only `E8001`/`E8152` wait for the write), emit options are a typed built-in schema, and an
    unknown emit target is `E8003`. Reason: `check` and `build` must report the same findings.

38. **`ItemElement` gets clean member names with legacy wire values** (`NONE = "_NONE"`…) rather
    than `@cpp(name:)` overrides. Reason: DECISIONS 3, and one fix for every target.

39. **Imports are per file** (as in Go); `E2005` applies within a file and against the package's
    declarations. Reason: game.items imports `studio` in two files, and file-local imports keep a
    file readable alone.

40. **`fail` is a predeclared identifier** (GRAMMAR §4.2 wins over TYPES §3.3); `warn` and `load`
    stay reserved. Reason: the grammar owns lexical categories, and the reserved-word set stays
    unchanged.

41. **`canon fmt` never moves a field's annotations to their own line when a method or check
    follows**, since GRAMMAR §3.1 rule 4 would read them as that item's prefix. Reason: keeps
    every formatted file a fixed point with one meaning.

42. **Generated-code texts:** the C++ conformance message keeps its `<package>: ` prefix
    (documented: every package prints to one stderr); the schema-mismatch error ends without a
    period in all three targets (Go's convention); TS test files write -2^63 as
    `-9223372036854776000`. Reason: one text per target, fixed by the spec.

43. **C++ layout is fully specified:** a table of every fixed comment per target, a
    declare-before-use order, and an include rule (every standard name a file uses, sorted), which
    adds `<cstddef>` to `pipeline.gen.cpp`; the missing docs and the store's `friend` were added to
    the goldens. Reason: goldens are byte-compared from M2.

44. **CLI text formats are exact:** `test` prints each failing `expect` with its location and
    `expected:`/`got:` lines, `explain` and `refs` separate columns by two spaces without padding,
    and the samples now come from the examples (resourcestudio's port, the moonstone refs).
    Reason: they are goldens (CLI-03).

45. **`Evaluate` has a JSON form**: lowerCamel tags in `api/canon.go`, every result key written,
    empty collections as `[]`/`{}`. Reason: the studio client and the API must agree at M4.

46. **`types.Kind` follows TYPES §2 exactly**; an asset is a `String` refinement, not a kind, and
    a keyed list is a `List` with `KeyedBy`. Reason: consumers need `Pair`, `VariantKind`,
    `DepUnion`, `None`, `Error`.

47. **rules.canon loads GuildTalentTree with `partial: true`** instead of typing its five unused
    keys. Reason: the rules package ports rules, not the talent-tree schema, as its other loads do.

48. **`ErrSyntax` is specified now and coded in the M0 split**; `api/doc.go` is added now. Reason:
    added alone, the sentinel is an exported name no other package uses, and an example test
    alone makes `deadcode -test` flag every API stub; both are new audit findings the ratchet
    refuses.

49. **Runtime helper texts are data files** (`runtime/*.txt`, embedded, written under their real
    names). Reason: DECISIONS 26 allows no exemption, and data is outside Go tooling and the
    auditor.

50. **`make check` gains `diag-check`, and `goldens-check` checks every MANIFEST now**; a
    written `canon.lock` is listed in its example's MANIFEST. Reason: no gate is a stub where
    something can already be checked.

51. **Text corrections to binding files:** DECISIONS 17 says "`if` (statements and expressions)
    … comprehension `if`" instead of the non-existent `?:`, and the triage line says 206 findings
    (AUDIT.md's recount). Reason: the wording was wrong, not the decision.

52. **The studio mockup stays out of git.** `meta/spec-phase/mockups/studio.html` embeds the full
    real item index (6,943 items) and real icons, so it is git-ignored under DECISIONS 29. It stays
    on disk and at the published private page. The small fixtures in `examples/_fixtures/` follow
    the model Louis approved and are committed. The working documents of the spec phase
    (AUDIT*.md, MOCKUP-GAPS.md, review/, mockups/) moved to `meta/spec-phase/` to clear the root.

53. **Rule thresholds are the limits, in `tools/audit/thresholds.tsv`, and only rise by a
    maintainer's commit.** The file holds every number a rule compares a measurement against
    (20 keys, gocognit's and dupl's limits included, which the stock lane renders into
    golangci.yml); detection parameters (how many words prove a decision comment kept, what
    counts as a one-word constant) stay constants, since they define a rule rather than bound it. Rows are `<key><TAB><value>`,
    every key exactly once, values positive integers; any other row stops the tool. A higher
    value is always looser, so `baseline-guard` fails a raise against the base revision.
    Reason: decision 26 wants limits as data, and data nobody guards is an exemption.

54. **`diag-message-inline` flags five things outside `internal/diag`, tests included**: a
    string literal (or `+` chain of literals) holding a code token; one holding a fixed run of
    a catalogued template of at least `diag-text-words` (4) words, or a §1.6 text; a `fmt` call
    in any argument of a diag function or method (types resolve stored builders); a literal
    with whitespace in such an argument; a hand-built `diag.Finding`/`Related`/`Def`/`Variant`/
    `Arg` or an assignment to a diag `Message` field. Comments may cite codes. Reason: each is a
    way to put English or a code beside the registry; 3-word runs ("does not exist", "out of
    range") would flag ordinary Go error text, and a hit at 4 costs only a rewording.

55. **`diag-code-untested` judges the codes the compiler reports; `diag-code-unreported`
    (observe) counts the rest.** A code is reported once non-test code names `diag.E3501`
    (or a runtime helper text holds it), and tested only by a `<CODE>_<n>.txtar` in its owning
    package's `testdata/findings/` whose `findings.txt` holds it (IMPLEMENTATION-PLAN §7.2); a
    Go test naming the constructor does not count. Reason: baselining the 298 untested codes
    needs a hand edit of the baseline (`--init` refuses an existing one, `--tighten` never adds,
    `baseline-guard` fails an added entry), so the ratchet starts at zero and every newly
    reachable code must bring its test; at v0.1 `diag-code-unreported` turns enforce, and the two
    rules at zero mean every code is tested (decision 27).

56. **`ignore-count` counts every suppression a pinned tool honours**: `sovaudit:ignore(-file)`,
    `//nolint`, `//lint:ignore`, `//lint:file-ignore`, `#nosec`, `//gosec:disable`,
    `//exhaustive:ignore`, `//revive:disable`, in comments only. `//canon:unordered` is not
    counted: DOCTRINE §5 requires it on an order-free map range, and counting it would forbid
    what the doctrine asks for. Reason: an uncounted syntax is a way around the ratchet.

57. **`diag.Variant.Template` keeps the escapes of ERRORS.md §1.2** (`{{`, `}}`, `\\`, `\n`):
    it is the text between the backticks, not its rendering. Reason: ERRORS.md §2.2 says
    "unescaped", but an unescaped template cannot tell a literal `{name}` from a placeholder
    (`W1604`, `E1623`), so neither the renderer nor the round trip could read it back.

58. **`diaggen` lives in `internal/diag/catalog` (reading, validation, generation) behind a thin
    `internal/diag/cmd/diaggen`, and writes two files**: `codes.go` and `codes_test.go`, the
    table that calls every constructor once; `make diag-check` regenerates both into a
    temporary directory and diffs them. Runtime codes get a variable with `Def()` and no `At`.
    It refuses more than ERRORS.md §2.1 lists: a code outside its section's range, a runtime
    code not owned by `gen` (or `gen` on another code), an argument named `span`, message rows
    out of the codes table's order, a malformed separator row; its retired set adds `E3014`
    and `E3319`. The package list is read from IMPLEMENTATION-PLAN §3, sub-packages its rows
    name included (`eval/std`, which ERRORS.md uses). Reason: one reader of the §3 table for
    the generator and the dependency test, every constructor exercised by a generated test,
    and each extra refusal is a catalogue that could not be what its author meant.

59. **The dependency rule as tested** (`internal/testkit/deps_test.go` over `go list -deps`):
    a package imports rows above its own or its own row (a package and its sub-packages; a
    directory belongs to its nearest listed ancestor, so `diag/cmd/diaggen` is `diag`'s);
    `testkit` imports anything; a module package in no row fails (so `api/vm` needs a row
    before it lands); `unsafe` and `C` are refused; every import outside the module and the
    standard library, test imports included, must be a package of the Choice column of §11
    (the `go.lsp.dev` fallback is not allowed until it is listed). The test stats every Go
    file so the test cache cannot hide a new import. Reason: the strictest reading of §3,
    §11 and `.claude/rules/go.md` §7 that a test can hold.

60. **Package names under `internal/gen/` are `jsongen`, `gogen`, `cppgen`, `tsgen` and
    `viewgen`; `internal/eval/std` is `std`.** Reason: `go` is a keyword, and `json` would
    shadow `encoding/json` in the very package that writes JSON; one suffix for all five.

61. **Skeleton packages carry an empty running Example** (`func Example()` with an empty
    `// Output:`) until their milestone gives them an API; no constant or sentinel is written
    before code uses it. Reason: `pkg-example` requires an Example now, and an exported name
    made only to be shown would be the premature surface `exported-but-local` forbids.

62. **Until M1, `cmd/canon` answers every invocation with one line on stderr and exit 2** (a
    usage error, CLI.md §2.5). Reason: a `main` package needs a `main` for `go build ./...`,
    and an empty `main` exiting 0 would claim success; the cobra tree lands with `internal/cli`.

63. **M0.3 writes only what `codes.go` needs of the M0.4 contracts**: `source.FileID`, `Pos`
    and `Span` exactly as IMPLEMENTATION-PLAN §4.1 sketches them, and in `diag` `TypeArg`,
    `ValueArg`, `Builder` (it records its constructor call) and `Message` (it holds its
    builder, so a nested message is rendered with the outer finding's file set). Rendering,
    `Report`, `Bag` and `Finding` are M0.4's, with the table test that renders every message.
    Reason: the generated constructors cannot compile without these, and nothing more is used.

64. **`fixturegen` follows an embedded list, `fixtures.tsv`**, with one method tonight,
    `defines` (the include guard and the listed `#define` lines, byte for byte, in the real
    header's order); it fails without `testdata-real/`, writes nothing unless every fixture
    extracts, and refuses a fixture tree over 300,000 bytes. The JSON and text fixtures stay
    hand-trimmed until a row-selection method is designed. Reason: DECISIONS 29 and
    IMPLEMENTATION-PLAN §7.3 (deterministic, fixed list, small), with no method guessed.

65. **CI is `.github/workflows/check.yml`**: `make check`, then `go test -race ./...`, on Go
    1.25 with `GOTOOLCHAIN=local` and a full clone (the baseline guard needs history). Reason:
    the module is published at github.com (DECISIONS 23); IMPLEMENTATION-PLAN §13.4 mentions
    GitLab for the benchmark runner, which stays open. `go.mod` now reads `go 1.25.0` and
    requires `golang.org/x/tools` for `txtar` (§11).

66. **An API stub with an error result returns an `*InternalError` (`ErrInternal`, message
    "unimplemented") instead of panicking; one without an error result still panics.** Reason:
    API.md X2 lets no panic cross the API boundary where an error can carry it, and the
    Examples must run through the entry points (decision 67).

67. **The API's Examples all run** (each has an `// Output:`): the ones for implemented helpers
    (constructors, error texts, `Version`) print real output; the others open `examples/` through
    `FindProject`, get the stub error and print nothing, so they need their real `Output` at M4.
    Their edits are `DryRun` and their builds `Check`, so running them never writes. Reason:
    `go test` lists only Examples with an output comment in the test main, so a compile-only
    Example is no root for `deadcode -test` and leaves every stub `dead-unreachable`.

68. **`api/canon.go` keeps its name and holds what IMPLEMENTATION-PLAN §12.4 calls
    `project.go`** (options, FS, Project, open and close, packages, revisions, overlays). Reason:
    API.md §1, CLI.md §5 and README.md link `api/canon.go`, and agents may not edit the first
    two; a rename would be three `dead-link` findings. It becomes `project.go`, the links
    pointing at `api/`, when Louis next edits those documents.

69. **API enum types are declared in `api/constants.go`, each above its constants**
    (`Severity`, `ValueKind`, `OriginKind`, `EditMode`, `Reason`, `RefKind`, `OpKind`,
    `ChangeKind`, `EventCause`, `Target`, `OutputStatus`), with `None`. Reason: `const-placement`
    puts the values there, and Go declares an enum type with its values.

70. **API doc comments are one line per exported declaration, citing the API.md rule; a field
    comment stays only for a fact API.md does not state; stub bodies are written as blocks.**
    Reason: `comment-ratio` (20%) and `comment-adr-narration` (a `§` only on a one-line
    comment); API.md is the documentation, and a one-line doc over a one-line body is 50%.

71. **`Version()` is implemented** (compiler `0.1.0`, language `0.1`, the three format
    strings, the commit from the binary's `vcs.revision`), and `ViewModel.Decode` wraps the
    JSON error with the package name. Reason: API.md §14 and CLI.md §3.14 fix every value of
    `Version`, whose Example must run; `wrapcheck` refuses an unwrapped error.

72. **`source` (M0.4) numbers files from 1 and normalizes on entry.** `NoFile` (0) is no file;
    `FileSet.Add` turns `\r\n` into `\n` (a lone `\r` and a BOM stay for the lexer's `E1124` and
    `E1123`), makes both paths `/`-separated, refuses content past `MaxInt32` bytes with
    `ErrFileTooLarge`, and is safe for concurrent `Add` and `File`. `Locate` resolves a `Span` to
    a `Location` (display path, line, column, end line, end column: API.md §1.3's shape) for
    `diag` and `api`; `Span` gains `Len`, `Contains`, `Cover`. `File.Path` is given by the caller:
    the `@root/...` display-path helper lands with `project` (M1), which knows the roots. Reason:
    §4.1's sketch, with the one error a byte offset of `int32` forces and no guessed root logic.

73. **Tokens and trivia (M0.4).** An interpolated string is several tokens: `STRING_HEAD`, the
    expression's own tokens, an optional `FORMAT_SPEC`, then `STRING_MID`… `STRING_TAIL` (the same
    four for multiline strings), so every node of an interpolation names tokens of `File.Tokens`.
    `DOC` lines are trivia (`TriviaDocComment`), not tokens; a BOM is `TriviaBOM`; a character no
    token starts with is an `ILLEGAL` token; the separator pass's `NL` is a zero-width token with
    no trivia. A token's `Trailing` is its spaces and comments up to the line break; the break and
    everything up to the next token are that token's `Leading`. Line-break counts are not stored:
    they are the line numbers of the two tokens. Reason: a flat, lossless stream (§4.1, GRAMMAR §10)
    in which FORMATTER §8.1's attachment is read off the tokens.

74. **The node contract (M0.4).** `Node` is `First`, `Last`, `Kind` and an unexported `children`
    method (the closed set, and the walker's per-type dispatch: `Walk`, `Inspect`, `Children`);
    `NodeKind` has one value per node type, printed as the type's name. `File` is the root node,
    so §4.1's field `Kind FileKind` is named `FileKind`. Nodes embed `Bounds{From, To}`; a node
    whose list brackets are not its own bounds records them as `Delims`; a `DocComment`'s bounds
    are its host token and `File.Span` gives its own lines. Reason: every node walkable and
    spanned, with no size-limit exemption for a type switch over 119 kinds.

75. **The node set differs from §4.1's list, each time as GRAMMAR.md requires.** Fourteen added:
    `AnnotationList`, `RecordBody` (records and cases share it), `AmendSegment`, `ProjectEntry`,
    `ProjectList`, `ProjectMap` (GRAMMAR §7: project values are not expressions), `ViewPlural`
    (`plural`, MOCKUP-GAPS 25), `ViewColumn`, `ViewFilter`, `TypeArgs`, `TypeArm` and `StmtArm`
    (arms typed by position, beside `MatchArm`), `ParenType` (FORMATTER §11 keeps parentheses),
    `ExprBody`; `Interp` and `File` are nodes too. Two removed: `ItExpr` (`it` is an ordinary
    identifier, GRAMMAR §4.2 over the list; `check` resolves it, `E2109`) and `MapComp` (GRAMMAR
    §5.12: a comprehension is a `BraceLit` with `Clauses`, as `check.Info.Literals` already
    assumes). Contextual keywords stay `IDENT` tokens; a node keeps them as a `Tok` or an `Ident`,
    and records a keyword alternative as a `TokenKind` (`CheckDecl`, `Pattern`, `CompClause`,
    operators) rather than a new enum. `Modifiers` holds `local`, `export`, `retired` wherever one
    may appear, for `E1133`. Reason: GRAMMAR §10 (one node per production, lossless) wins on
    behaviour; the production table test holds every GRAMMAR production mapped to node kinds and
    every kind to a production.

76. **Literal nodes carry exact values.** `IntLit` a signed `*big.Int` (unary `-` folded),
    `FloatLit` the decimal `Coef × 10^Exp` (a `big.Rat` of `1e999999999` would exhaust memory),
    `DurationLit` signed milliseconds, `StringLit` decoded parts, `RegexLit` the body with `\/`
    replaced. A shorthand lambda's body is its postfix chain, rooted at a `SelectorExpr` whose `X`
    is nil; `else if` chains are an `ElseIf` field. Reason: GRAMMAR §2.4–§2.7 (the lexer holds
    exact values, nothing rounded) with a bounded representation.

77. **`types` (M0.4) is a representation with its text form, no judgment.** `R(args)` is
    `*types.AppliedRecord` of kind `Record` (TYPES §2 lists `R(args)` under `Record`) and
    `F(args)` a `*types.TypeAppType` of kind `TypeApp`, where §4.2 folded both into one struct.
    Type arguments are resolved stable paths (`types.Arg`: parameter, earlier field or map binder,
    then fields), and a type function holds its resolved `Scrutinee` and `Arms`, not the syntax of
    its `match`: verify, the IR and the fingerprint read them without resolving again. `Underlying`
    and `Base` strip the top layer only; a `Refined` holds one written refinement (range, pattern,
    `where`, asset) and a refined named type nests them; an `Alias` has no parameters (that is a
    `TypeFunc`); `LitUnionType`'s alternative is `Of` (a field `Base` clashes with the method).
    Collections are interned by the checker: two refs have the same target exactly when their
    `*Collection` pointers are equal. A field carries its wire mapping resolved (`Wire`,
    `WirePath`, `Inline`, `NoneWire`, `Unit`, `Enc`, `Pairs`), so `JSONCase` and `HasDefault` are
    gone; target annotations stay raw (`Annotations`) for `ir`. `Identical`, `Assignable` and
    `Join` (§4.2) land with the checker in M1, as sketched: written now they would be stubs.
    Reason: the strictest typing of §4.2 that compiles, with nothing dead.

78. **Canonical type text.** Names are package-qualified; a ref prints its collection
    (`ref teamboard.statuses`), or its element type when it is resolved in an enclosing record
    (`ref resource.rules.TalentNode`), so two refs into different collections never print alike
    (`E3309`); a `where` prints its source text; the unwritable kinds print `Kind(V)`, `Pair(A, B)`,
    `F(*)`, `none`, `_`, and the error type `invalid`. `FloatText`, `DurationText` and
    `QuoteString` (STD-06) live in `types`, the lowest package that prints them (refinement
    bounds), and `value` uses them. Reason: TYPES.md prints types in Canon syntax without fixing
    these cases, and one text form avoids a second formatter.

79. **`value` (M0.4): pointer values, the pointer is the instance.** Copies share the instance
    (EVALUATION §4.2), so once-per-instance checks and invalid marks key on the pointer; poison,
    invalidity and taint are evaluator state, never value fields. Every typed value holds
    `T types.Type` (a declared type may be an alias or refined). Added to §4.3: `CaseKind` (the
    value of `v.kind`), `Pair`, `Symbol` (a bare identifier given to a dependent field, kept
    until verification resolves it, TYPES §11.4), `Identity.Owner` and `Ref.Owner` (a collection
    held by a field is one per enclosing instance), `Prov.MoreFrames` (EVALUATION §13's count of
    omitted frames), `Key.Text`, `Map.Get`; a record's input-field slot is nil. `Equal` expects
    converted operands: a ref against a value without identity is unequal (the evaluator
    dereferences first), and a value type of another package (the evaluator's function values)
    equals only itself. `value.Text` is dropped: `CanonText` is the one way. Reason: identity
    and immutability as EVALUATION §4 states them, with no field the evaluator could mutate.

80. **`ir` (M0.4): pointers, typed options, per-instance results.** A `TypeRef` points at the IR
    type (`Named`, of this package or an imported one) instead of naming it, so the fingerprint
    walks the type graph across packages; `Type.QName()` is computed from `Pkg` and `Name` (a
    `QName` field clashes with the method). `$schema` is per emitted value (`Value.Schema`,
    FINGERPRINT §2), not per record. Emit options are typed (`GoPackage`, `Namespace`, `Target`,
    `Mode`), no map. A field keeps `WirePath` only, and its constant `Default`, with `Computed`
    for a default that reads other fields (`E8014`). Methods keep one `Instance` per receiver
    (result, or lookup table); package fns a `Value` or a `LookupTable` (domains in domain
    order, dense cells); translated fns a typed `PExpr` body (statements folded into `Let` and
    `If`, enum members as `Lit`), their `Reads` of self, parameter and result ranges, and
    `Vector`s with the Go and C++ expectation beside TypeScript's. A dependent type keeps its
    discriminant (parameter, wire path, type), its branches in arm order and each member's
    branch (`NoBranch` for Never), which is what CODEGEN §5.6 and FINGERPRINT §4.4 read. An
    input field keeps its own range and pattern (CODEGEN §5.12), and the package lists the
    define tables its refs target, sorted by name (CODEGEN §5.8). `ir.File` and `ir.Generator`
    fix §4.5's generator signature. Reason: every fingerprint input and every construct of
    CODEGEN §5 is carried, typed, without map order reaching an output.

81. **`diag` (M0.4) resolves spans through a `diag.Files` interface, not `*source.FileSet`**:
    `Path(id)`, `Position(id, pos)` and `Content(id)`; a file whose `Path` is `""` is no location
    (how a `NoFile` span renders). `NewBag(files, pkg)` and `Render(w, files, findings, opt)`
    take it, and `Render` returns an `error` wrapping `ErrWrite` (§4.4's sketch has none, but a
    writer can fail). `*source.FileSet` satisfies it with three one-line methods over `File(id)`,
    or `build` adapts it. Reason: M0.4 let `diag` import only `FileID`, `Pos` and `Span`, and an
    interface keeps every renderer test free of a file set.

82. **`diag.Finding` gains `MoreFrames` and `Reads`; the builder gains `MoreFrames(n)` and
    `Reads(fields)`.** `Stack(frames)` keeps the 16 innermost and counts the rest; `MoreFrames`
    adds the frames a provenance already cut (EVALUATION.md §13 keeps "a count of omitted
    frames"). Reason: F13 prints `(<n> more frames)` and F5 writes `reads` (API.md §4.1,
    VIEWMODEL.md J15), neither expressible with §4.4's field list.

83. **The `Bag` is a view, not a state machine**: it keeps `DefaultMaxFindings` (1000) until
    `Truncate(n)` (a negative `n` keeps none); `Findings()` and `Summary()` always return the
    sorted (F2), deduplicated (EVALUATION.md §14; "the first produced" is the first added) and
    truncated (F7) view, whatever the call order; `Summary()` counts the dropped findings and
    names them in `Truncated`; `Summary.Merge` adds counts and sorts `Truncated` by package.
    There is no exported `Sort`: `Render` sorts a copy of what it is given (F2 across packages).
    Reason: no result depends on when `Truncate` or a late `Report` happened.

84. **Text-form details API.md §4.4 leaves open**: every line is trimmed of trailing spaces (F9's
    "No line ends with a space" read as a rule of the whole form, so an empty line of a
    multi-line message is empty); a related location or frame without a file writes no location
    (`expected by (check)`, `in f`); `(<n> more frames)` has no singular (F13 writes one form);
    a negative duration renders as 0; `RenderOptions.Golden` writes `(…)` in the text form only,
    the JSON summary always writes `ms`; the JSON summary line is
    `{"summary":{"errors","warnings","packages","ms"[,"truncated"]}}`, `truncated` last (CLI.md
    §2.4 "gains"). Reason: the strictest reading of each rule, and one byte-exact output.

85. **One JSON string codec, in `diag`**: `AppendJSONString` writes WIRE.md §7.3's form (invalid
    UTF-8 as U+FFFD) for findings (F5), `canon.lock` values and built paths; `UnquoteJSON` reads
    an RFC 8259 string strictly (a control character, invalid UTF-8 and an unpaired surrogate
    escape are refused, as WIRE.md §3.2), for `edit` and `lock`. `diag` is the lowest package all
    of them import; `jsonsrc` and `wire` should reuse it. Reason: `encoding/json` escapes `<`,
    `>`, `&`, U+2028 and U+2029, which WIRE.md §7.3 forbids, and three encoders would drift.

86. **Argument renderings ERRORS.md §1.3 leaves open**: a `Pointer` argument is a plain RFC 6901
    pointer (like `Finding.Pointer`), rendered `#` plus the pointer percent-encoded as an RFC 3986
    fragment (RFC 6901 §6), so the sample of `builder_test.go` is now `/modelTypes/3`; `Expr`
    replaces each run of space, tab, LF or CR by one space without trimming; a nil `Type` or
    `Value` renders empty. The note templates of §1.5 are constants of `diag`, held equal to
    ERRORS.md by `TestNotesFollowErrorsMD` (the generator does not read §1.5). Every one of the
    438 messages renders (435 through their constructors, the 3 runtime codes from typed
    samples), pinned by `internal/diag/testdata/messages.txtar`. Reason: "`#` then the pointer"
    is a rendering, so the argument is the pointer itself.

87. **Edit paths (M0.4, API.md §6.1)**: `KeyLit` gains `Raw`, the key as written, so `String()`
    prints exactly what `Parse` read (`Parse(s).String() == s`, fuzzed); `Text` is the word or
    the decoded string, `Int` the integer; a key built without `Raw` prints in its kind's form.
    `[-0]` is accepted (the grammar allows `-` then `0`) and reads 0; a key or position past
    `int64`/`int` is `ErrBadPath`; words are ASCII with `_` a letter and `_` alone refused (SPEC
    §2.4). `edit.ErrBadPath` is `edit`'s own sentinel, which the API maps to `canon.ErrBadPath`
    (`edit` cannot import `api`). `Step`, `Resolved` and `Resolve` wait for M4: they need
    `types.Type`, `value.Value` and a snapshot. Reason: an exact round trip is the strictest
    reading of "as parsed" and what §7.7's fuzz target checks.

88. **Reading `canon.lock` (LOCK.md §2.4) tolerates exactly what it lists**: `Parse(id, data,
    pkg, bag)` reports `E6005` per bad line and returns `(file, ok)`. A blank line is empty or
    spaces only; separators are runs of spaces (a tab is not one), and a leading or trailing
    space fails the line; a JSON string is one field, spaces included; a missing final LF is
    read. A line starting with a conflict marker is `merge`, even before the header. The first
    other non-blank line must be the header: `# canon.lock v<n>` with `n` a canonical numeral
    other than 0 and 1 is `version` and stops the reading (a newer format is not judged);
    anything else is `header`, that line is not read as a fact, the next ones are; no non-blank
    line is `header` at offset 0. A first field other than `table`, `enum`, `field` is `kind`
    (a repeated header is kind `#`); a name whose segments are not identifiers is `syntax`, a
    well-formed name of another package or with a segment too many or too few is `package`;
    enum codes are integers; `-0` reads 0; an integer past `int64` is `syntax` (TYPES.md §7.2
    caps `UInt64` at `int64`). Reason: every tolerance of §2.4, nothing more.

89. **The `canon.lock` model**: a `File` is a set in canonical order (LOCK.md §2.3: kind bytes,
    written name bytes, value, holder; integers before strings should one name hold both); a
    table fact's value is its key, so it sorts by holder; identical facts merge, `retired`
    wins, and a merged fact keeps its first `Span` (E6002 names "the second lock line"). `Format`
    is byte-exact on LOCK.md §9.1 and §9.6 (their stated sizes and SHA-256 checked by the test
    that reads them from LOCK.md) and on `examples/teamboard/expected/canon.lock`. The per-code
    cases `internal/lock/testdata/findings/E6005_<n>.txtar` hold a `findings.txt` section, which
    lock's test compares and rewrites under the golden harness's `-update` (`golden.Run` writes
    a `want` file, while IMPLEMENTATION-PLAN §7.2 names the section `findings.txt`). Reason: the
    set semantics of §2.4 and goldens written by the tool.

90. **`project` schema types (M0.4)**: `Project{Name, Canon, Roots, Languages, Studio, Budget,
    GoModules}`, `Roots` and `GoModules` in name order, `Root`, `GoModule` and `Studio` with the
    span that names them (for `E1009`, `E1012`); `New(name, canon)` fills the defaults; `Budget`
    0 means not declared and the evaluator's default applies (IMPLEMENTATION-PLAN §12.4 names the
    default budget in `eval`); `DefaultLanguage` is `en`; the supported versions are `0.1`. No
    parser and no validation before M1. Reason: §7.1's schema as data, with no default guessed
    twice.

91. **`*source.FileSet` is a `diag.Files`** (closing DECISIONS 81): `Path(id)`, `Position(id,
    pos)` and `Content(id)` over `File(id)`; an unknown id is no file (`""`, `0:0`, `nil`). The
    interface check and a test that a file set renders text and JSON byte for byte like the
    renderer tests' in-memory files are in `diag`'s external test, which may import both.
    IMPLEMENTATION-PLAN §4.4's sketch now shows `Files`, `NewBag(files Files, pkg)` and
    `Render(…) error`. Reason: one position source for `build` with no adapter, and a renderer
    whose tests need no file set.

92. **The golden harness names the expected file by option**: `golden.Expected(name)` on `Load`
    and `Run` (default `want`), recorded in `Case.Expected`; an empty name is refused. `lock`'s
    per-code cases run through `golden.Run(…, golden.Expected("findings.txt"))`, so the
    `flag.Lookup("update")` copy of the harness (DECISIONS 89) is gone. Reason:
    IMPLEMENTATION-PLAN §7.2 names the section `findings.txt`, and a second compare-and-rewrite
    loop per package would drift from the harness.

93. **Spec sketches and TYPES §2 follow the code, meaning unchanged**: IMPLEMENTATION-PLAN §4.2's
    `LitUnionType` has `Of` (DECISIONS 77) and §4.5's `Record`, `Enum`, `Variant`, `Dependent`
    have `Pkg, Name` with the `QName()` method (DECISIONS 80), since a field and a method of one
    name do not compile. TYPES §2 no longer lists `Refined` and `KeyedList` as kinds: a
    refinement is a layer with its base type's kind (paragraph under the table), and
    `[T] keyed by f` is written in the `List` row, as DECISIONS 46 already decided.

94. **API error texts have one form (API.md X1)**: `"<op N: ><path: ><sentinel text><: detail>"`,
    each part only when present, so every text holds its sentinel's text. `*SyntaxError` now
    starts with `syntax error: ` (it printed the location alone); `*ProjectError` and
    `*SyntaxError` write their first finding as `<file>:<line>:<col>: <message>` (the message
    alone without a file); `*StaleError` and `*NotCanonicalError` join their files with `, `
    (they printed Go's `[a b]`, ambiguous for a path with a space); `*RejectedError` writes
    `1 error` / `n errors` (it wrote `n error(s)`). `*InternalError` keeps its field `Msg`:
    renaming it `Detail` would change the frozen contract. X1 now states each type's detail;
    `api/errors_test.go` pins every type's text. Reason: the spec's intent (one format, the
    sentinel visible) read strictly, with no text left to a type's habit.

95. **API.md and IMPLEMENTATION-PLAN §12.4 match the split API**: §8.8 says `Edit` has its JSON
    form through struct tags and `Op` implements the JSON interfaces (as coded); V4a points at
    `api/evaluate.go`, `api/edit.go` and `api/findings.go`; §2.1 states that a relative `Roots`
    directory is relative to the project root (the field comment is dropped, DECISIONS 70);
    `Value.Len` counts a keyed list's elements and a variant's current-case fields (the kinds of
    `ValueKind`); §1.1, §1.2 and §15 speak of `api/` rather than the pre-split file. §12.4's
    `api` row lists `constants.go`, says `build.go` holds `Format`, `FormatJSONSource` and
    `Version`, and that `canon.go` is its `project.go` until DECISIONS 68's rename.

96. **`api/vm` has a row in IMPLEMENTATION-PLAN §3, between `i18n` and `views`, consuming
    nothing**, and a skeleton package now (`doc.go`, empty running Example, DECISIONS 61).
    Reason: the lowest row that still lets `views`, `gen/view`, `api` and `cli` import it; with
    the row, M0.2's "every package of §3" holds only if the package exists, and
    `TestViewModelRow` keeps its directory out of `api`'s row.

97. **ERRORS.md §2.2's `Template` comment reads "as written between the backticks: escapes kept
    (DECISIONS 57)"** instead of "unescaped". Reason: DECISIONS 57 wins; the Louis-call is closed.

98. **M0.1, M0.2, M0.3, M0.5 and M0.6 are ticked; M0.4 is not.** IMPLEMENTATION-PLAN §6 M0 asks
    for the eight contracts of §4, and §4.7 (`check/info.go`) and §4.8 (`eval/host.go`) are not
    written (meta/plan.md's M0.4 line listed six; it now lists all eight), and the consumer
    approval waits for the M0 review. Reason: a box is ticked only when its acceptance passes.

99. **The M1 lexer and parser start before the M0 gate closes.** They consume only the `source`
    and `syntax` contracts, which are complete and committed. The review of M0 and the last two
    contracts (§4.7, §4.8) run in parallel. If the review changes the syntax contract, the parser
    follows it; no other M1 work starts before M0 is accepted. Reason: the night's time budget.

100. **The checked program (M0.4, `check/info.go`) reaches every identifier.** `Info.Uses` keeps
    `*syntax.IdentExpr` (names in value position); `NameUses` holds every other `*syntax.Ident`
    that names an object without declaring it (type names and qualified-name parts, patterns,
    `is` targets, `.name` like go/types, record-literal fields, named arguments, amend segments,
    `at`, `keyed by`, import names, view names); `ObjectOf(node)` reads `Defs`, `NameUses` or
    `Uses`, so rename and find-references have one entry (M0 review MF-1). `Symbols` marks the
    identifiers kept as symbols (a dependent value's, TYPES §11.4; a load `format:`), so an
    `IdentExpr` is in `Uses`, `Keys` or `Symbols`. `Object` gains `File()` (a `Tok` means nothing
    without its file: findings, provenance, go-to-definition) and `Pkg()` is "" only for
    built-ins. `Folder.Fold` takes the `owner` object of the expression: folding reports
    `E4101`, `E4102`, `E4301` and needs the package's bag and the file. `Conversion` gains `Key`
    (map keys, a pair's first) beside `Inner`, since a map converts keys and values; `ToList`
    covers `table T` and keyed lists to `[T]` (a `value.Table` is not a `value.List`).
    `MatchInfo.Covers` numbers `none` `NoneIndex` (-1). A `Bad*` recovery node makes the
    declaration holding it broken; a `BadExpr` or `BadType` is typed `types.ErrorType` (TYPES
    §1) and nothing else is recorded; a top-level `BadDecl` declares nothing. `Bags` is
    `map[string]*diag.Bag`, not an interface: it calls nothing below `check`, so DECISIONS 34's
    two seams stay two. The kinds print the names of §4.7's sketch, which a test reads from the
    plan. Reason: a contract no consumer must widen in M1, with every conclusion typed.

101. **`check.Check`, `check.Bags`, `eval.Options`, `eval.NewFolder`, `eval.New` and the
    `Evaluator` land with their logic in M1**; M0.4 writes the data and the seams (`Program`,
    `Package`, `Info`, `Object`, the kinds, `Folder`, `Host`, `Root`), and §4.7/§4.8 keep the
    exact signatures. Reason: DECISIONS 77 and 87 — a body written now is a stub that lies (a
    `Force` returning "poisoned" with no finding) or panics, and an exported name used by no
    running entry point fails `exported-but-local` and `dead-unreachable`.

102. **The evaluator's surface (§4.8), fixed for M1**: `Host` as sketched; `Options{Budget,
    Layers}` (`Budget` 0 is the default 10⁸, DECISIONS 90; `Layers` the active ones in stack
    order); `NewFolder(bags, opt)`, since a fold reports findings; `BeginVerification(ctx)` runs
    stage B itself (`host.Verify` on each value forced so far, in completion order, EVALUATION §5)
    before a first `Force` verifies too, so `build` needs no completion list; `MarkInvalid` and
    `Invalid` hold the invalid marks `verify` finds and `rules` reads (EVALUATION §7.3), which
    are evaluator state (DECISIONS 79). Reason: `Verify`'s bool cannot say which sub-value is
    invalid, and the invalid set must live where the taint is decided.

103. **`load` arguments are constant expressions.** The path, `at:`, `prefix:`, `format:`,
    `partial:` and `header:` of every `load` form must fold to constants during checking (`check.Folder`).
    Reason: the build manifest, caching and determinism need the set of files read to be known
    before evaluation, and `Host.Load` receives the syntax node (DECISIONS 34). A non-constant
    argument is a type error. WIRE §6.1 gains this sentence at the next spec pass.

104. **Constant folding during checking spends the step budget.** It is one counter per invocation
    (EVALUATION §12). Folding is evaluation, so it is charged the same way; nothing is free.

105. **Findings are sorted in a total order; the duplicate kept is the least (M0 review MF-4).**
    `Bag.Findings()` and `diag.Write`/`Render` sort by the F2 key, then by every other field
    (severity, end position, package, path, pointer, check, layer, related and stack element by
    element, cut frames, reads), then the raw span; of duplicates (EVALUATION §14) the least is
    kept. This replaces DECISIONS 83's "the first produced is the first added", which made the
    result depend on which goroutine reported first. `Builder.Stack` and `Builder.MoreFrames` are
    setters, the finding counting both (a second `Stack` no longer double-counts). EVALUATION §14
    says so. A test reports duplicates in 64 shuffled orders from 1 to 8 goroutines and asserts
    identical findings, text and JSON. Reason: NFR-05 (Workers=1 and Workers=8 identical), and a
    "first" that scheduling decides is no rule.

106. **API findings are written by `diag`, through a resolved form (M0 review MF-5).** `diag`
    gains `Located` (a finding with every span resolved to a `source.Location`), `Locate(files,
    findings)`, `Write(w, located, opt)` (what `Render` now calls) and `(*Located).AppendJSON`.
    `api` converts `canon.Finding` to a `Located` (the only place outside `diag` that builds one:
    a resolved record, not a new diagnostic, so DECISIONS 54 is kept) and exports
    `WriteFindings(w, findings, WriteOptions{JSON, Summary, Duration, Golden})`, API.md F16:
    `cli` prints findings through it, so M1 acceptance 2 (teamboard's `findings.txt`) and every
    JSON line come from the one writer (DECISIONS 85). `canon.Finding` gains `MoreFrames`, written
    as the F5 key `moreFrames` after `stack`, omitted when 0: a JSON-hidden count would make
    `--format json` lose a line the text form prints, and F6's round trip lossy.
    `Finding.MarshalJSON` and `UnmarshalJSON` are implemented (the latter refuses an unknown key
    or severity). The `diag`→`canon` conversion lands with `Check`, its first caller. Reason:
    `cli` may not rebuild `diag.Finding`, and two writers of one format drift.

107. **`edit.Step` and `edit.Resolved` exist; `Snapshot` is `edit`'s own concrete type (M0
    review MF-2).** `Step` and `Resolved` compile now, as §4.6 sketches them. `Snapshot` is not
    an interface: it is the part of one build's results `Resolve` reads, which `workspace`
    assembles from `build`'s results (`build` sits above `edit` in §3, `edit` may import it) and
    hands to `edit`, so there is no third injection seam (DECISIONS 34 holds).
    `Resolve(s *Snapshot, p Path)` and `Snapshot`'s fields land with `build`'s result type. A
    declaration now would be a stub with guessed fields. `edit.SyntaxError{Offset, Reason}` is
    the typed parse error (review S-EDIT-1): it unwraps to `ErrBadPath` and to the reason, so
    `api` fills `PathError.Detail` without cutting strings. This narrows DECISIONS 87's deferral.

108. **`ir.Emit` carries its resolved output (M0 review MF-6).** `Dir`, the output directory (a
    `ts` emit's: its file's), project-relative through the roots `project.canon` declares,
    `/`-separated and clean; `FileName`, a `ts` emit's file; `GoImport`, a `go` emit's import path
    (CODEGEN §2.8). Stage E fills them, the imported packages' emits included. A `--root`
    override is never used for them, so redirected roots (tests, the determinism job) produce the
    same code. `ir.File.Path` is relative to its emit's `Dir`. Generators never resolve roots:
    `<GoImport>/rt` and an imported emit's `GoImport` are Go imports, and a C++ or TS relative
    include is the path from one `Dir` to another. Reason: a generator is a pure function of
    `(ir.Package, ir.Emit)`, and `Out` as written cannot give an import path or a relative
    include.

109. **`lock.Add` accepts exactly what `Parse` reads back from `Format` (M0 review MF-9).**
    `Add(fact) (changed bool, err error)` refuses, with `ErrBadFact` and the set unchanged:
    - an unknown kind;
    - a name that is not `<package>.<identifier>` of the lock's package;
    - a field fact without one identifier field, or another kind with one;
    - a table fact with any value;
    - an enum fact with a string;
    - a value mixing an integer and a string, or a string that is not valid UTF-8;
    - a holder that is not an identifier;
    - a retired field fact.

    `Parse` merges without re-checking. The property "Add accepts ⇔ the fact's `Format` line
    parses back to that fact" is a table test over 20,000 random facts and a fuzz target
    (`FuzzAdd`, 9M executions clean). Reason: the strict option, refusal over normalization. A
    build must never write a lock it refuses to read.

110. **`maprange` is an audit rule, not a `go vet` analyzer (M0 review MF-10).** Rule
    `maprange` (enforce), lane `determinism` in `tools/audit`, run by `make check`
    (`audit-check`). It flags every `range` over a Go map, and over `maps.Keys`, `maps.Values` or
    `maps.All`, in `api`, `build`, `diag`, `edit`, `format`, `gen`, `i18n`, `ir`, `jsonsrc`,
    `lock`, `views`, `wire` and the packages under them, test files included (an Example prints).
    `//canon:unordered <reason>` on the loop's line or the line above exempts a loop. In any
    package, a marker with no reason, or with no map range on its line or the next, is a
    finding. A test holds the package list equal to IMPLEMENTATION-PLAN §7.5, which now says
    this; §6's milestone gate and §3's `testkit` row ("analyzers") follow. Three existing test
    loops were sorted or marked. Reason: the audit already type-checks the module and gates
    `make check` with enforce and ratchet modes. A `-vettool` binary would be a second checker
    to build and pin, and the `x/tools/go/analysis` dependency it needs is not in §11.

111. **API should-fixes of the M0 review.**
    - `Version().Commit` is the compiler's revision, never the embedding program's (API.md T3):
      the stamped `vcs.revision` when the compiler is the main module and `vcs.modified` is not
      true; the revision of a `github.com/fantasim/canonlang` pseudo-version when a program
      depends on it; `""` otherwise.
    - The Examples open `examples/` from an in-memory `canon.FS`, a copy of the repository's
      tree, with every root of `project.canon` redirected: fixtures for the read roots, memory
      for the written ones. `FindProject` has its own Example.
    - `Revision` is implemented in the same change as `Open`, so no Example reaches its panicking
      stub. This corrects DECISIONS 67's "at M4": `Open` may land before M4.
    - `EvalResult.Headings` is printed in key order.

    Reason: review S-API-1 and S-API-5, where Examples would read machine paths and print in map
    order.

112. **`.gitignore`'s `coverage.*` no longer hides Go source.** `!coverage.go` follows it: the
    pattern had kept `tools/audit/internal/lanes/diagnostics/coverage.go` out of every commit, so
    a clean clone's `make audit-self` did not compile. The file's pre-existing nesting finding,
    now visible to the audit, is fixed (a helper `addCodes`). Reason: the gate must pass on what
    git holds.

113. **Spec texts follow DECISIONS 105-111.**
    - API.md: §4.1 `MoreFrames`; F5 `moreFrames`; F6's refusals; §4.5 `WriteFindings` (F16);
      T3.
    - CLI.md §2.4: a `moreFrames` row.
    - EVALUATION §14: the kept duplicate.
    - IMPLEMENTATION-PLAN:
      - §4.4: `MoreFrames`, `Reads`, `Located`, `Locate`, `Write`, the order; the stale
        `TestEveryCodeIsTested` now points to `diag-code-untested`, as DECISIONS 55 decided;
      - §4.5: `Emit`, "generators never resolve roots";
      - §4.6: `KeyLit.Raw`, `SyntaxError`, `Snapshot`, review S-EDIT-2;
      - §7.2 (`diag-code-untested`), §7.5, §6, §3's `testkit` row.

    ERRORS.md §2.3 still names `TestEveryCodeIsTested`; left for Louis, since ERRORS.md feeds
    diaggen.

114. **Wire encoding, the fingerprint and baked Go generation also start early.** They consume
    only `types`, `value` and `ir`, which are committed and reviewed (M0 must-fix items closed for
    them). Generators work from hand-built IR fixtures, as IMPLEMENTATION-PLAN §4 intends. The
    checker, evaluator, verifier and CLI wait for the parser and M0's acceptance. Same reason as 99.

115. **`wire` encodes a whole data file from verified values, and reports no finding.**
    `wire.Document{Schema, Kind, V, Fns, Methods}.Encode()` writes WIRE §8.2's bytes. The encoder
    reads everything from the values and their `types` (field wire paths, units, encodings,
    markers, tags); `$` keys arrive through `Methods func(*value.Record) []Fn` and `$fns` as `[]Fn`
    (a result, or cells over finite domains), since `wire` may not import `ir` (§3). A value with
    no encoding (not a whole unit, equal to its none marker, a repeated bits member, two keys with
    one text, a range, an unresolved `Symbol`) is refused with a sentinel error, never written:
    E8102 and E3317 are reported by the verification pass `wire` owns, with their txtar tests,
    once values come from sources. Float text reuses `types.FloatText` (DECISIONS 78), so there is
    no `wire/float.go` (§12.4). Reason: the IR's values are verified; one encoder, two callers.

116. **A top-level value is written as `rows` when its declared type is a list, keyed list or
    table, and as `value` otherwise, `[T]?` included** (`null` or the array). WIRE §8.2 says "when
    the value is a list"; a data-mode loader must know statically which member to read, and a
    `none` has no rows. `Document.Kind` is the declared type's kind (`ir.TypeRef.Kind`).

117. **A dependent type is not a named type for `unit:`, `int` and `unit=`.** FINGERPRINT §4.3 names
    records, variants and enums; a Duration in a dependent arm takes the field's unit, in the
    fingerprint and in the encoder alike.

118. **The fingerprint is `ir.Fingerprint(t, fns)` and `ir.Schema(pkg, name, t, fns)`.** The caller
    passes the package fns only for the value whose file carries `$fns`; translated fns are
    skipped. A `DepMap` `TypeRef` holds its key's `ref` type in `Key`; an integer `TypeRef` holds
    its width (`Bits: 64, Signed: true` for `Int`, as `types.Basic`), and `Bits: 0` is refused
    rather than read as `Int`. The vector test reads the ten texts from FINGERPRINT.md §7 itself.

119. **A file that carries `$fns` writes them even over an empty domain.** WIRE §8.3's "same table
    with no entry" shows no `$fns`, while its `$schema` (vector 9's) covers `canTransition`; the
    encoder follows §5.11 and §8.2 and writes `"$fns": {"canTransition": {}}`. The sample's text
    needs Louis's confirmation (a one-line spec fix).

120. **Go naming follows CODEGEN §3.2's rule, not its table's `IIWeaAxeAngel`.** GoCap upper-cases
    a word only when it is in the closed initialism list; `II` is not, so `II_WEA_AXE_ANGEL` is
    `IiWeaAxeAngel` in Go too (lowerCamel `iiWeaAxeAngel`, which the table agrees with). The row
    contradicts the rule it illustrates; Louis confirms the one-cell fix. `gen/go` computes every
    generated name in scopes (package, method set, struct, data, parameters) and refuses a
    collision or a non-identifier with the sentinels `ErrNameCollision` and `ErrName`: E8005 and
    E8011 belong to `ir` (ERRORS.md `Package`), whose stage-E validation reports them as findings,
    and a generator returns an `error`, not findings. Reason: the rule is the normative part, and
    the audit counts a code as tested only in its owning package.

121. **Baked Go's reference layout (CODEGEN §6.2 is abridged).** One `<last>Data` struct built by
    `build<P>` through `sync.OnceValue`; the builder first allocates every container's rows,
    then assigns each entry, so a resolved ref is `d.<value>.At(i)` and any entry can point at any
    other, itself included; a record value that is an entry of an emitted table is that entry.
    Record-typed fields and values are stored as `*R` (§6.1). A resolved single ref stores only
    the entry, and its key getter reads the entry's `id` (or keyed-list key), as `GetInitialStatusID`
    does; `[ref T]` stores both lists. Storage names the spec does not fix carry an interior `_`
    (`hint_ok` for the presence flag of an optional, `next_ids`, `status_id`, the lookup local
    `night_i`), which lowerCamel of a Canon name never produces, so they cannot collide with a
    field. A variant is `{kind, value any}` and `As<Case>` is a type assertion. Enum `String` and
    `Wire` of a value that is no member return Go's `Role(9)` form; `Parse<E>` returns `(0, false)`.
    Baked mode has no schema constant (nothing is read). An empty package doc leaves only T1.

122. **Lookup tables are nested arrays indexed by each parameter** (`[6][6]bool`,
    `canTransitionTable[from][to]`), not §6.2's flat `[36]bool` read at `int(from)*6+int(to)`: an
    out-of-range argument then panics instead of reading another row's cell. A `Bool` parameter
    is indexed through a local, a `@codes` enum through a switch onto its declaration index (-1,
    so a panic, for a value that is no member). A table whose cells point into the baked data is
    built from it once through `sync.OnceValue`; any other is a plain variable. A precomputed
    package fn is the one-cell case (`answerTable`). An optional result that nil cannot mark is a
    `{v, ok}` cell. Reason: the strict option, and one code path for methods and package fns.

123. **Literals keep their Go type and value exactly.** A Duration is always `N * time.Millisecond`
    (`0 * time.Millisecond` too, so `const NoWait` is a `time.Duration`, not an untyped 0); a Float
    is the shortest text that reads back at its width, and -0.0 is `math.Copysign(0, -1)`; a -0.0
    constant is refused (`ErrUnsupported`), since no Go constant holds it. A list literal longer
    than 80 bytes puts one item per line (the reference layout). An import is named only when its
    package name differs from its path's last element; two imports of one name are a collision.

124. **What baked `gen/go` does not emit yet is refused, never skipped** (`ErrUnsupported`):
    translated fns and their conformance test (M2, CONFORMANCE.md), dependent types (their values
    need the discriminant read from a sibling field), input fields and `LoadInputs`, a table-typed
    field (CODEGEN §4.2 names its id type `<id type>` without saying which), a baked value of a
    record or variant of another package (its fields are unexported there), export fns of a case
    without fields (it has no type to hold them), and a lookup parameter ref into another
    package's table. Modes other than `baked` are M2's. The id enum of a table exists for every
    public table value; its container, accessor and resolved refs only when the emit selects it.

125. **`internal/gen/go/runtime/rt.go.txt` holds CODEGEN §6.3 verbatim, as IMPLEMENTATION-PLAN
    §12.4 places it, although the audit then reports its eight codes as untested.** The text
    signals E3201, E3202, E3204, E4101–E4104 and E4108, which `diag-code-untested` counts as
    reported and wants tested in their owning packages (`check`, `verify`, `eval`, `eval/std`),
    none of which exists yet. Moving or renaming the text to escape the rule would be an evasion;
    the findings clear when those packages land their per-code txtar cases (M1). Until then
    `make check` fails on them alone, so `gen/go` lands with or after those tests.

126. **`gen/json` reports no finding; what the checker, stage E or `build` reports is an error
    there.** ERRORS.md gives `emit json`'s codes to other packages (E8150 and E8009 `check`, E8151
    and E8153 `ir`, E8152 `build`), `ir.Generator` returns `([]File, error)`, and `ir.Emit` and
    `ir.Value` carry no span. `jsongen.Generate` refuses what those packages refuse first, so it
    never writes a layout the loaders cannot read: file mode with other than one value or a name
    not ending in `.json` (`ErrFileMode`), `values` naming no public value or one twice
    (`ErrValues`), a value of a data-mode code emit or an `@reload` value not written here as
    `<value>.json` (`ErrDataMode`), a type with no wire form (`ir.ErrFingerprint`). One emit writes
    into one directory, so E8153's "same directory" holds by construction. E8152 is not checked:
    `build` finds it in phase 8 over every output of the build (CODEGEN §2.1), so `Generate`
    returns both files of `values: [hp, HP]`. Reason: the audit counts a code as tested only in
    its owning package (as decision 120 for `gen/go`), and a generator's finding has no location.

127. **A json emit in file mode names its file in `Emit.FileName`, `Dir` being its directory,**
    as a `ts` emit does; `FileName` is `""` in directory mode, and stage E fills it when `out`
    ends in `.json`. IMPLEMENTATION-PLAN §4.5 and `ir.Emit`'s comment say "ts" only, and `Out` is
    for messages, so the generator had no way to know `potions.json` (decision 108); the comment
    needs the one-word fix. An empty `Emit.Values` is the default (every public value, declaration
    order), as `gen/go` reads it, so stage E must refuse or expand an explicit `values: []`. The
    first value in the emit's order carries `$fns`.

128. **`$schema` is computed by `gen/json`; `Value.Schema`, when set, must equal it
    (`ErrSchema`).** The first value's covers the package's stored fns (decision 118), so stage E
    fills `Value.Schema` the same way, or the data-mode loaders, which compile it in, refuse the
    file. A record or case value's `$` keys are its declaration's stored methods (found by
    `Pkg.Name`, or variant and case, over every type the package, its values and its fns reach),
    each result the `Instance` whose `Recv` is the encoded record itself (pointer identity). An
    encoded receiver without an instance, an instance listed twice, a precomputed fn without its
    value or a lookup without its table is `ErrFn`; the instance of a receiver this emit does not
    encode is ignored, since instances cover every receiver of the package (EVL-02).

129. **The syntax contract after the M0 review (MF-7, S-SYN-1 to S-SYN-6).** `FloatLit` is
    `±Coef × 10^Exp` with `Neg` (the folded unary `-`: `-0.0` is negative zero), `Coef ≥ 0` and
    `Exp int64`; a float whose exponent, counted from its last written digit, does not fit an
    `int64` is `E1110` (GRAMMAR sets no bound: a gap for Louis). `Tok` is 1-based:
    `File.Tokens[0]` is a `BOF` sentinel (new kind `TokBOF`, no trivia), `NoTok` is 0, so an unset
    `Tok` field reads as absent. The walker skips a typed nil held by an interface field, and
    `File.Span` of a nil node is empty at the file's start. `File.Trailing(n)` adds the trailing
    trivia of a separator comma directly after `n` (`a: 1, // c`). New exports: `Parse(src, kind,
    bag)`, `kind` being `FileProject` for the project's `project.canon` and anything else for
    other files (source, layer and translation files are told apart by their first tokens), and
    `LookupWord` / `IsNameable` for printers. A comprehension `BraceLit` holds one `MapItem`, a
    `name:` key becoming an `IdentExpr` (TYPES §5.2); in `project.canon` the doc block is
    `ProjectDecl.Doc` and `File.Doc` stays nil. Reason: the review's findings, closed the strictest
    way that keeps the tree lossless.

130. **The tree after a syntax error (M0 review MF-8): recovery nodes.** `BadExpr` (also an
    annotation value and a project value), `BadType`, `BadStmt`, and `BadDecl` (also a record,
    variant, view, group and brace-literal item). Invariants: no required field is nil; an
    expression or type that does not parse is a Bad node of its category; a node whose own
    structure (a name, a bracket, a keyword) is missing becomes a Bad node where its list's element
    type allows one, and is left out of a list of one concrete node type (`[]*EnumMember`,
    `[]*Param`, `[]*Arg`…); a Bad node covers the tokens skipped or, when none was, is empty at
    the unexpected token (`Last == First-1`), and a node that consumed no token is empty too
    (`File.Span`, `Leading` and `Trailing` of an empty node are empty); every node lies within
    its parent and after its previous sibling; Bad and empty nodes appear only in a file with an
    error; an import after a declaration, a second package clause, a project declaration in a
    source file and an item after a comprehension's clauses are reported and left out of the tree.
    The parser resumes at the next separator of the innermost `{ }` list, or at a top-level
    declaration keyword or `@` in column 1 whatever brackets are open, and reports one `E1116` per
    item (none on an `ILLEGAL` token or an interpolation the lexer reported). Reason: the review's
    stricter and more useful option; an empty node never overlaps a neighbour.

131. **Token boundaries and trivia (M0 review S-SYN-4, completing DECISIONS 73).** A string piece
    spans its delimiters: `STRING_HEAD` from the quote (a multiline head with the rest of its
    opening line) through the `{`, `STRING_MID` from the `}` through the next `{`, `STRING_TAIL`
    from the `}` through the closing quote; `FORMAT_SPEC` is the `:` and the spec, not the `}`; an
    `Interp` spans its expression and its spec. A token's `Trailing` is the trivia after it up to
    the first line break (a block comment that starts on its line trails it even across lines);
    the rest up to the next token is that token's `Leading`; the file's first token leads with
    everything before it, a BOM included. `NL` is empty, at the border of the previous token's
    trailing and the next one's leading. A line break between two tokens is any `\n` between them,
    comments included.

132. **Lexical choices GRAMMAR leaves open.** `#` is a token only directly after `[`, with no
    trivia between. `///` that is not the first text of its line is an ordinary comment and
    `W1001`. The leading-zero rule applies to an integer part and to each duration segment
    (`1h05m` is `E1110`), not to fraction or exponent digits. Outside literals and comments a
    control character is `E1106`; a `\r` is `E1124` anywhere. A regex body is a literal:
    non-ASCII allowed, control characters `E1124`. A line break or a comment inside an
    interpolation is `E1112`: a plain string ends there, a multiline one goes on with its text.
    The separator pass pops a closer's opener wherever it stands in the stack and closes every
    interpolation at a line break. Nesting past 1,000 levels is `E1116` (expected `nesting`), so
    no input exhausts the stack. Reason: the strictest reading that keeps recovery sound.

133. **`E1116` wording.** Expected names are GRAMMAR symbols bare (`expr`, `type`, `IDENT`, `WORD`,
    `stringLit`, `topDecl`, `pValue`…) and fixed tokens quoted (`","`, `")"`, `"else"`); the found
    text is the token's first line, `NL` for a separator. After an item, a token that can start
    another item on the same line is `E1117` and read as the next item; anything else is `E1116`.
    The hints GRAMMAR §5.3 and §5.6 attach to `E1116` ("a widget takes value and siblings",
    "write field <name>") have no ERRORS.md variant, so plain `E1116` is reported (a gap for
    Louis).

134. **Annotation checks in the parser (GRAMMAR §8).** The catalogue is data
    (`internal/syntax/annot_catalog.go`) checked per site, each argument with its own sites: an
    argument not allowed at the site is `E1118` for its annotation; an unknown flag or positional
    value is `E1104` naming the value as written; positional arguments are named in messages by the
    catalogue's placeholders (`wire`, `T`, `why`, `n`, `tpl`, `menu`); `@since(0)` is `E1119`
    kind integer; a studio argument must be one word (`E1119` value, naming its studio enum); a
    bare `@json` is accepted; `@cpp(header:)` needs `struct:`; a duplicated or out-of-order argument
    is `E1121`, a duplicated annotation `E1120`. Type-dependent rules stay with TYPES and WIRE.

135. **Reserved words as package segments stay `E1125`; two examples break it.** GRAMMAR §4.3 lists
    packages among the names a reserved word may not take (ERRORS.md has the `PackageSegment` kind
    for it), but §11 says every example parses, and `examples/features/match` and
    `features/retired` declare `package features.match` / `features.retired`. The parser follows
    §4.3, their AST goldens record the `E1125`, and M1 item 1 is therefore not met for those two
    files. Louis: rename the two packages (QA owns `examples/features`) or allow reserved words as
    package segments.

136. **The parser's goldens and tables.** `internal/syntax/testdata/ast/<example path>.txt` holds each
    example's tree, `== findings`, then its findings' text form; `testdata/parse/*.txtar` the same
    for focused cases (section `ast`); `testdata/findings/<CODE>_<n>.txtar` a case per code. A tree
    prints one node per line, two spaces per level: `<Field>: <Kind> [name] [attr=value …]
    @<line>:<col>-<line>:<col>`, `Field[i]` in a list; an `Ident`, `IdentExpr` or `QualifiedName`
    prints its name; literal values, operators, keywords (not `TokInvalid`), present `Tok` fields
    (their text, not `OpTok`) and true flags are attributes; a string's text parts are `text "…"`
    lines between its interpolations; `Delims` are left out (the span holds them). The plain-text
    AST goldens are rewritten under the golden harness's `-update` flag, read through
    `flag.Lookup`, since `golden.Run` rewrites txtar sections only. Token sets are built by
    `tokenSet(…)` in `tables.go`, the precedence table and the names staying in `constants.go`,
    which must stay under 500 lines (DECISIONS 26).

137. **Reserved words stay reserved in package names; the examples are renamed.** `features.match`
    and `features.retired` become `features.matching` and `features.retirement`, so GRAMMAR §4.3
    (E1125) keeps its strict form. Closes Louis-call 3a.

138. **M0 is accepted.** All ten must-fix items of `meta/reviews/M0-review.md` are closed: MF-1 and
    MF-3 by the check/eval contracts and the close-out, MF-7 and MF-8 by the parser, and the rest by the
    must-fix commit. The eight contracts exist. M0.4 is ticked.

139. **The M1 command line uses the standard `flag` package, not cobra** (departs from
    IMPLEMENTATION-PLAN §3 and §11, Louis-call 6). Adding cobra changes `go.mod`, which CLAUDE.md
    reserves to Louis; `flag` covers `version`, `init`, `new` and `check`. The tree lives in
    `internal/cli` (`cli.Main(ctx, args, Env)`); `cmd/canon` only reads the working directory,
    wires SIGINT/SIGTERM to the context and exits. Flags go before or after the command and among
    the arguments (`--` ends them); every CLI.md §2.3 flag is global; a flag error, `-h` and
    `help` included (CLI.md lists no help), is `canon: <error>` and the usage, exit 2. `--layer`,
    `--color`, `--lang` and `--watch` are not defined yet (exit 2): layers are applied from M3,
    and `E1901` lands with `--layer`. Reason: a new dependency is Louis's call.

140. **Where findings belong before checking.** A file's parse findings go to the bag of the package
    its `package` line names. A file without a package line joins no package (it is in no unit,
    `Packages`, selector or Checker input): its findings go to the bag of its directory's package
    (`a/b` → `a.b`) when that package exists, else to the project's own bag, package `""`, with
    `project.canon`'s, `E1012` and an `E7003` override. The own bag counts as no package in
    `Summary.Packages` and is not truncated by `MaxFindings`. `project.Reader` parses each file
    with a bag of its own and again into its package's bag only when it has findings. Reason:
    bags are per package (F7), a package is known only once its files are parsed, and no finding
    is ever moved between bags.

141. **Project-file details GRAMMAR §7.1 leaves open.** A root name and a `go_module` key must be
    written as an `IDENT` (a string or a reserved word is `E1007` name, resp. `E1009` root); a
    `go_module` key given twice is `E1005` key; a module path holding any Unicode space is `E1009`
    path; a `languages` item written as a string is `E1006`, a dotted one `E1008`; a version whose
    numbers overflow is `E1001`. A value or key the parser already refused (a Bad node, or a
    string with an interpolation, `E1132`) gets no second finding, and a root whose path was
    refused still counts as declared for `go_module`. An `X:` prefix and a leading `\` are
    absolute on every platform (`E1007`, `E7001`). A `--root` naming an undeclared root is
    `E7003` with no location, and `Open` fails with `ErrProject` (API.md §2.1); with no root
    declared its message ends `roots:` (the renderer trims the space; the JSON form keeps
    `roots: `), a gap in ERRORS.md recorded in Louis-call 6. Reason: the strictest reading, and
    every refusal carries a catalogued finding.

142. **A missing project.canon is a `*ProjectError` wrapping `ErrNoProject` with its `E1003`
    finding** (departs from API.md §15, which gives `ErrNoProject` no error type; Louis-call 6).
    `FindProject` finds none upward; `Open`, `--project` included, finds no file named exactly
    `project.canon` in its directory (listing, case-sensitive, IMPLEMENTATION-PLAN §10). The
    typed error keeps `errors.Is` and lets the CLI print `E1003` through `canon.WriteFindings`
    (DECISIONS 106), exit 2. For `--project` the message's "or its parents" overstates the
    search, since ERRORS.md has one variant (Louis-call 6). `Find` lists each ancestor directory
    (IMPLEMENTATION-PLAN §10); an ancestor it cannot list gives a wrapped I/O error.

143. **M1 snapshots, selectors and exit codes.** `Open` reads and checks `project.canon`, places the
    roots and scans the file set (API.md O2: `.` directories skipped, byte order), parsing no
    other file. Every call (API.md S1) reads `project.canon` again (new errors fail the call as
    they fail `Open`, a `*ProjectError`, which `Check` and `Packages` may thus return although
    API.md R3 does not list it: a departure, Louis-call 6), rescans, reads and parses every
    `.canon` file, and reports only the selected packages (R2). `Revision()` refreshes from the
    raw bytes on disk whether `project.canon` checks or not; a file, or the listing, it cannot
    read enters S3's listing as `<path> NUL unreadable` (`.` for the listing), so a broken or
    missing file still changes the revision (S3's read set is the files read; `canon.lock` and
    loaded files join with their readers). `Options.Layers` is checked against
    the loaded packages by `Check` and `Packages` (O4) but not applied before M3. Selectors:
    `./x`, `../x`, `.`, `..`, absolute paths and `*.canon` are paths, made project-relative by the
    CLI, anything else a name; a directory selects the one package the files directly in it
    declare (an ancestor included); a directory without files of its own selects the package
    named after it, and none when no package has that name, even if its subdirectories hold files
    (`./game/items/entries` with files only in `IK1/` is unknown); files of several packages are
    a usage error (exit 2, no code); an unknown selector is named as typed, found against one
    `Packages` listing (`project.Match`). `-q` hides warnings, the summary
    still counts them; `--max-warnings` gives 4 only without errors. An I/O error is returned
    wrapped and exits 2, although API.md R3 does not list it and CLI.md §2.5 has no I/O exit code
    (departs from both, Louis-call 6); `ErrInternal` exits 3 with the report line, an interrupt
    130. Reason: correct before incremental (NFR-01 memoization is M4's workspace).

144. **`init`, `new` and `version` outputs.** `canon init` names the project after its directory
    unless `--name` (an `IDENT`, else exit 2), writes `project acme {` / `canon: "0.1"` / `roots {}`
    / `languages: [en]` / `}` (the canonical layout), adds `.canon/` to `.gitignore` unless a line
    holds it, and prints nothing; `--project` names an existing target directory. `canon new`
    takes lowerCamel `IDENT` segments (W1003's convention, E1125's reserved words refused), opens
    the project first (so `--project` must hold a valid `project.canon`), creates the directories
    under the project root and `<last>.canon` holding `/// Describe package <p> here.` then
    `package <p>`, refuses an existing file and prints nothing. Paths in their messages are
    project-relative (CLI.md §2.1). Both write through `os.Root` with modes 0600/0750 (gosec).
    `canon version` writes `(unknown)` for an unknown commit in text, `"commit":""` in JSON.
    Reason: CLI.md §3.1, §3.2, §3.14 leave these open.

145. **The phase-2 seam is `build.Checker`**: `func(ctx, *project.Project, []*syntax.File,
    map[string]*diag.Bag) *check.Program`, i.e. `check.Check` with its `Folder` bound. Nil skips
    phase 2, as in M1 until `check.Check` and `eval.NewFolder` exist; then `build.Open` defaults it
    to `check.Check(ctx, p, files, bags, eval.NewFolder(bags, eval.Options{Budget: p.Budget}))`.
    `api` stays a thin adapter over `build` until `workspace` exists (M4). Reason: DECISIONS 34's
    two seams, with no import of an unfinished package.

146. **Stage B re-walks each top-level value against its declared types, as a safety net.**
    `verify.Verifier.Check` starts from the let's declared type (from `check.Program`, since a `Dur`
    or `Member` carries none) and follows field, element, key and entry types, never refs, checking
    refinements, Float32 overflow and finiteness, retired members and cases, refs, keyed-list keys,
    `@stable` values and assets. It does not replace the conversion checks of EVALUATION §4.3: a
    value converted earlier gets the same finding again, and §14 keeps one. Each finding marks
    exactly the value it is located at (`MarkInvalid`); `Result.Valid` is false then. The evaluation
    forms of `E3201` (sized integers, stored Durations) and `E3101` (loaded tables) are not
    `verify`'s: `E3201` is met at conversion (EVALUATION §4.3, §6.3), with `eval`/`check`.
    Reason: stage B holds for every value whatever conversions ran, reporting only verify's codes.

147. **Verification choices EVALUATION §5 leaves open.** A ref into a poisoned collection gets no
    finding and is marked invalid (§7.2). An unbound level-1 ref is marked invalid and returned in
    `Result.Unbound` (walk order, with its path) for `build`'s `eval.Host` adapter to report
    `E3505`, which ERRORS.md gives to `eval` though §3.4/§5 meet it in stage B; §4.8's `bool`
    cannot carry it yet (Louis-call 7). `E3502` fires only inside a live
    table entry; any enclosing retired entry allows anything (LOCK §4.3). Assets are checked clean
    path, then extension, then existence, only the first failure reported; nil `verify.Assets`
    finds no file. A hard error in a stage-B `where` re-run stops the walk and sets
    `Result.Poisoned` (§7.1: the root is aborted; the surface has no `Poison`, so the caller
    poisons). Related locations come from `Info.TypeExprs` inverted for the types built where they
    are written (refinements, refs, lists, maps, tables, optionals, unions: the checker must record
    those pointers), from the enum or case declaration for `E3506`, and from `@codes(…)` for
    `E3102` codes; the bound of `E3204`/`E3206` is the first non-regex argument. Dependent types
    (`E3801`, `E3802`) wait for M3: `value.Record` holds no bound parameter values. Reason: the
    strictest reading that reports nothing twice.

148. **The evaluator surfaces `verify` and `rules` consume.** `verify.Evaluator` is `Force`,
    `MarkInvalid` and `Where(ctx, *types.Predicate, it) (holds, ok)`, a re-run at no step cost
    (the conversion paid it); `rules.Evaluator` is `Invalid` and `Run(ctx, *syntax.CheckDecl, self)
    rules.Run{Aborted, Failed, Message, Reports}`, one run of one check, `self` nil at package
    level, aborted for a broken record's check. Both are consumer-side interfaces EVL implements;
    §4.8 has neither. `rules` places findings: an instance at its provenance span, `at f` at the
    field's value (path `.f`), a package check at its keyword, fail/warn at their `at` value with
    its path (no location for none), related to the whole check declaration; `Reads` from
    `Info.Uses`/`Selections`. Broken checks and records (`Info.Broken`) never run. A missing bag is
    `ErrNoBag` in `verify` and `rules`; a `Runner` serves one goroutine. Reason: §4.8 names only
    what `build` wires; the placement rules are `rules`'.

149. **Comparing `canon.lock` with the sources (LOCK §4).** `lock.Sources` holds the current facts,
    each located at its entry, member or `@stable` value; `Skip(kind, name)` marks a collection
    that exists but is poisoned or broken, never compared (nor its fields). `File.Verify` reports
    per collection in canonical order. A gone key is `held` when a key the lock does not know now
    holds one of its `@stable` values (`KindStableValue`, first in canonical order), else `renamed`
    when exactly one key went and one came, else `removed`; a gone member is `held` (`KindCode`)
    or `removed`. A value or holder the lock itself records is never reported again (no finding
    beyond the conflict). A conflict names the collection (two holders of one value) or
    `collection.holder` (two values of one holder) and quotes both lines. `W6006` counts only new
    values (unknown holder and unknown value); retirements do not count. `Update` only adds.
    `E6004` needs `eval`'s layer application. Reason: every violation reported once.

150. **`check.Check` folds only through the Folder it is given.** A nil `bags` or `fold` is API
    misuse: `Check` returns nil (IMPLEMENTATION-PLAN §4.7 keeps the signature, so there is no
    error to return); `build` binds `eval.NewFolder`. The literal folder is a fixture of `check`'s
    tests only (§4.7: "tests of `check` pass a fixture folder that knows only literals"). A fold
    that fails without a finding in the owner's bag is `E3015` at the expression, so no
    declaration breaks silently. Reason: the evaluator stays the one evaluator.

151. **What imports bind.** `import p` binds the last segment of `p` (SPEC §3's `import shared.ui
    // use qualified: ui.Tone`), `import p as q` binds `q`, `import p { A }` binds only `A`. Every
    import has an `ObjPackage` object: `Defs` of its alias, `NameUses` of its path's last part;
    the earlier parts name no object. An import binding a name the package declares is `E2005` at
    the import, `first` the declaration. Reason: TYPES §3.1 says "binds p" for a dotted `p`.

152. **What `check.Info` records where §4.7 leaves it open.** A qualifier of a qualified form is in
    `Uses` (type or package), a type qualifier's `Types` is the named type, a package qualifier
    has none. Built-in methods and members have one `ObjBuiltin` object per name (`Selections`,
    `NameUses`); `Define.value` has a field object. An entry selection of a collection whose keys
    are dynamic puts the `SelectorExpr` in `Keys`; its key names no object, like a built-in's
    parameter names, a load's form and options and an `expect` outcome. `Callee.TypeArgs` are the
    STDLIB type parameters in binding order (the receiver's `T`, or `K` and `V`, first);
    `Overload` is the row's index among the rows of its name in the receiver's STDLIB table; a
    recorded signature prints `Seq(T)` as `[T]`. An inner link of a `?.` chain has its type with
    the `?.` unwrapped; only the chain's end is optional (TYPES §6.5). `fail(…)`/`warn(…)` have
    type `none`; an entry's object has its element type, a type function's `DepUnion(F)`, a
    widget's its value parameter's type. Reason: every node answered once, nothing invented.

153. **Resolution details TYPES leaves open.** Level 1 of `ref T` (§10.2) counts the record whose
    field holds the ref as containing itself, and containment follows tables too. `.id` and
    `.retired` are members of a record exactly when some table of the program has it as its
    element (the condition of `E2105`). `E2102`'s hint is the other name in scope at the smallest edit
    distance, 1 or 2 and below the typed name's length, ties by byte order. `E2101` names the member qualified by its
    type's declared name (`Tone.warning`), as the user writes it. Reason: the strictest reading
    that keeps every example well typed.

154. **Typing details TYPES leaves open.** An optional field without default is not required in a
    literal (absent is `none`, as WIRE §5.4 decodes it); a spread anywhere suppresses `E3302`.
    `table T` and `stable table T` are statically identical. The literal rows of §6.4 (`[]`, `{}`,
    `none`, an integer literal with `Float`) are the checker's, `types.Join` does the rest. In a
    comparison the context-dependent operand is read from the other's type (a name contextually,
    a numeric literal as `Float` when the other is one); a right operand that is not is
    synthesized, but a name or a list, brace or lambda literal is read against the left's type; a
    ref compared with its entry type is dereferenced (`Conv` `Deref`); a literal union compares
    with its literals and with its alternative. `for k in m` is `E3018` with an empty second name;
    iterating a non-iterable is `E3002` against `[_]`; indexing a type with no index is `E3007`
    with operator `[]`; `x op= e` with `e` optional is `E3402`; calling an optional function
    value is `E3402`. Reason: one reading per construct, the strictest code that fits.

155. **Literals are checked against refinements statically, with verify's codes.** A scalar literal,
    a constant string and a list literal's length checked against a refined type report `E3201`,
    `E3204` or `E3205` in phase 2 (TYPES §5.3, §7.4), although ERRORS.md gives `E3204`/`E3205` to
    `verify`; the declaration is then broken, so verification never repeats them. `E3204`'s bound
    is the refinement's source text. Reason: TYPES requires the static report.

156. **`E3804` covers every operator and conversion of a dependent value** (§11.4 "anything
    else"): its `{op}` is the operator, or the expected type of a conversion (`Int is not
    available on a dependent value (P(*))`). Reason: §11.4 names passing one as `ref items`.

157. **Constant expressions (TYPES §15, DECISIONS 103).** They may name their own binders (lambda
    parameters, comprehension variables); `if`, `match`, `load` and `self` are `E3015`. A `load`
    argument that is not constant is `E3015` with the kind `ConstValue`: ERRORS §1.4 has no kind
    for load arguments (gap for Louis).

158. **`values` is an option of every emit but `view`.** CODEGEN §2.1's bullet "`values` (default:
    every public value) selects the values emitted" and `resource/vocab`'s `emit cpp { values: …
    }` win over the table's column, which lists it for `json` only. Reason: the example is law.

159. **`W1002` is reported for every loaded package; `W1003` waits.** GRAMMAR §9.1 does not limit
    `W1002` to the selected packages (every example is documented). `W1003` is "on declarations of
    the selected packages only" (§9.2), and `check.Check` does not know the selection: it is not
    reported in M1 (gap for Louis: a selection for `Check`, or `W1003` in `build`).

160. **Layers without imports.** `E1909` is reported when another loaded package declares the
    amended name (a layer file has no imports), a built-in name is `E1902`; `E1908`'s path starts
    with the let's first field (`paths.port`, `[k]` as written, `[#n]`). Reason: TYPES §3.1's
    per-file imports make "a name of another package" otherwise unreachable.

161. **Gaps the checker leaves strict.** `_` outside a widget parameter and a lone literal type are
    `E3002` although no expected/found pair fits the message (it prints `_`). A map literal key
    written `name:` that is a symbolic key of a collection with dynamic keys, or of a dependent
    key, and an amend segment `.k` naming a new key or an entry of a table with dynamic keys,
    have no `Info` entry: `Keys` and `Symbols` are keyed by expression and those keys are
    `Ident`s (contract gap). A record parameter whose type is neither a record nor a ref has no
    code (TYPES §11.1). A `@codes` member without a value is `E3201` with the value `none`; a
    type-function result refinement naming a parameter is `E3803`'s `argument` variant (§11.2
    has no variant of its own).

162. **`E7109` locates the offending character and quotes it.** The span is the first character
    at which the text stops being JSON (zero width at the end of the source), the `detail` is
    that character written as a JSON string (WIRE.md §7.3, `""` at the end), and the pointer is
    the innermost open array or object. Reason: ERRORS.md gives `E7109` one `Text` argument, and an
    English detail would be message text outside `internal/diag` (DECISIONS 27); gap for Louis:
    variants such as `eof` and `depth` would read better (`JSON syntax error: ""` at the end).
    Spans are in the content `source.FileSet` normalized (`\r\n` read as `\n`), so a raw
    `\r\n` inside a string is reported as `"\n"` at the LF.

163. **`jsonsrc` returns `E7105`'s cases; `load` reports them.** ERRORS.md gives `E7105` to `load`,
    so `jsonsrc.Parse` returns an `*EncodingError` (span, and the unpaired surrogate or 0) and
    reports nothing: a UTF-16 or UTF-32 byte order mark (its bytes), the first byte that is not
    UTF-8 (the whole file is checked before its syntax), a `\u` escape of a low surrogate, or of
    a high one not followed by exactly the `\u` escape of a low one. The span is in the
    normalized `source.File.Content` (`\r\n` read as `\n`); `load` computes E7105's `{offset}`,
    a byte of the file, from the raw bytes. Reason: one reader, the owner reports.

164. **A JSON source with a repeated key has no tree.** Every key equal after decoding to an earlier
    key of its object is one `E7104` (at the repeat, `first` the first key, the pointer the
    repeat's value) and `Parse` returns `ErrDuplicateKey` without a tree; repeats before a syntax
    error are not reported, so a file that is not JSON has exactly one `E7109`. Reason: which
    value wins is undefined, and FORMATTER.md §14.1 leaves such a file unchanged.

165. **`jsonsrc.Format` writes a number's text as read.** FMT-02 canonicalizes the numbers read into
    Canon fields (WIRE.md §7.2) and keeps the others; `jsonsrc` does not know the types, so the
    caller sets `Node.Text` of each typed number first. `Format` writes no BOM. The layout equals
    WIRE.md §7.4 `pretty`, which `wire` implements again (`node.pretty`): a later cleanup can make
    `wire` build a `jsonsrc` tree. A node's pointer is built on demand from its container links
    (`Node.Pointer()`), never stored, so a tree's memory stays linear in its source. Reason:
    `jsonsrc` depends on `source` and `diag` only.

166. **The formatter's entry points.** `format.Source(src, kind, bag)` parses with the file
    kind the caller gives and returns `ErrSyntax` when `bag` holds an error of that file other than
    `E1123` (a lone BOM is removed, FORMATTER §1); `format.File(tree) ([]byte, error)` prints a
    tree and returns `ErrSyntax`, never panics, for a tree holding a `Bad` node or an empty node.
    The tree does not record findings, so an error reported without leaving such a node (a lexer
    error, `E1117` between two items on one line, `E1133` on a misplaced modifier, …) is refused
    only by `Source`, which reads them; `File` must be given a tree whose parse reported none.
    Reason: the caller (`api.Format`, `canon fmt`) builds its `*SyntaxError` from the bag, and only
    it knows whether a `project.canon` is the project root's.

167. **Comments sit outside their node's groups.** A node's own-line and trailing comments are
    printed around the node's groups, so a trailing comment breaks the lists enclosing its item but
    never the item itself (FORMATTER §8.2 keeps `id: String @json("dwID") // …` on one line); it
    still counts toward its line's width, as §7.1's `text(" // …")` says. In a `( )`/`[ ]` list
    the comma precedes an item's trailing comment. Reason: §7.1 does not say which group owns a
    comment's hard break; inside the item it would move every commented field's annotations.

168. **Comment placement FORMATTER §8 leaves open.** A comment ending a line inside a
    construct, or an own-line comment there, puts the break before the next token: an operator
    starts the continuation line with its operand (`a // c` / `  + b`), an `else` goes one level
    deeper with its block, whose `}` aligns with it, and one ending the line of an `else` does the
    same for the `{` or `if` after it (§3); other own-line comments start a continuation
    line one level deeper and the token after one takes its indentation. A one-line block comment
    followed on its line by code stays inline before it (§8.2 over §8.1), with one space on each
    side of every inline block comment; a block comment spanning lines after a token leads the next
    token, with what follows it on its line, on a line of its own; comments that shared a line keep
    sharing it. Every line of a block comment loses its trailing blanks (§2; "byte for byte" is
    read as "not re-indented"). A dropped comma leaves its comments to its neighbours: its trailing
    ones follow the separator written again after the token before (after the item in a broken
    brace list) when the comma shared that token's line, else they lead the next token on a line
    of their own. The colon of `key: { … }` in `project.canon` is kept when a comment follows it.
    Reason: the strictest reading that keeps the tree, every comment in order at its token, and
    idempotence (fuzzed, and tested by injecting a comment at every position of every example).

169. **Layout details FORMATTER §6–§7 leave open.** Hugging applies to a lone positional argument
    with no comment on its parentheses or at its edges. A postfix chain is its primary expression,
    the steps before its first `.`/`?.` step (kept on the head's line, §3), then its other steps;
    with two calls or more a break point stands before every `.`/`?.` step, field steps included.
    An empty statement block is `{}` and does not break its if chain; a comment between a list's
    brackets breaks it (§6.1). Type aliases and match-type arms use rule A; expressions inside a
    type (refinements, `where`, dependent-map domains) are flat, like annotation arguments, whose
    comma precedes a trailing comment and whose own-line comments are indented one level. Rule A
    step 3's "fits on a new line" measures the value flat at the deeper indentation followed by
    the rest of that line, as §7.1's `fits` does. A range whose upper bound starts with `.` keeps a
    space after the operator (`a...x` would lex as `...`). The names in an import's braces keep no
    blank line (§4's import block). Reason: the literal readings; the range space is the one spot
    where "no spaces" would change the tokens.

170. **Groups holding a single-line bit keep their holder's mode in `fits` (FORMATTER §7.1).**
    §7.1 counts a group met after the one being decided as flat unless it holds a hard line break.
    With §6.1's single-line bit that is not idempotent (§1), as fuzzing showed: on one line and too
    wide, `fn f(a: A) -> A { … }` breaks its parameters, the block counting flat, then the block
    breaks too; the second run finds the block broken and keeps the parameters flat. A group that
    holds a brace list, a block or an if chain (a group whose layout reads a single-line bit) is
    counted in the mode of the command holding it, broken if it holds a hard line break; every
    other group follows §7.1 literally, so a later width-only group still counts flat (`(a, b) =>
    value` breaks its parameters when the flat value does not fit). `fits` meets a broken rule-A
    group as the printer does (step 3 ends the line after the operator). All examples stay fixed
    points, and a 10-minute `FuzzFormat` run on the final code (13 552 013 executions) finds no
    non-idempotent input. Gap for Louis: §7.1's parenthesis could read "a group met in rest that
    holds a brace list counts in the mode of the command holding it; any other counts as FLAT
    unless it contains a hardline".

171. **Expression details the part-B review settled (TYPES §5–§12).** Parentheses are
    transparent: checked once, a parenthesis's `Types` is its content's, `Conv` sits on the
    innermost node, facts see through them. A brace literal against a dependent union (§11.4) is
    classified against the first branch that classifies it, in declaration order (the body, then
    the arms): §5.2 has no row (gap). A join (§6.4) types only a branch's own `none`, `[]` or
    `{}`; one nested deeper is `E3008`. `[].sum()` without an expected type is `E3314`.
    `s.matches(e)` with a non-regex `e` is `E3007` (`operator matches is not defined for String
    and T`), a parameter given by position and by name `E3004` `many` at the named argument, a
    binder in an arm of several patterns `E3603` at the binder, `Int` ordered against `Float`
    `E3310` with the right operand's type: no code or variant names them (gaps). A free built-in
    type parameter prints as `_`, a bound one as its type; a constraint failure is reported at
    the argument that bound it (`E3403` when it holds for `T` of a `T?`, `E3310` for an ordering
    constraint, else `E3002` against a type that meets it). `E3002`'s found side for a literal
    is its source text when it fits one line of 40 bytes, else `{ … }`. Reason: one finding per
    mistake, at the node written.

172. **Declaration details the part-A review settled.** A cycle between `const`s breaks them with
    no finding: `eval` finds the chain through their initializers' `Uses` and reports `E4301`
    (EVALUATION §3.2), which `check` does not own. An amend path steps through `T?` (`none` is
    `E1905` when applied, §9.3) but never through a `ref` (`E1905`: §9.2 has no row); `E1908`
    compares paths of the same let; a second layer file of one name is still checked; `[k]` on a
    table or keyed list reached by record fields reads a bare `k` as a key. A dependent value is
    assignable only where the same type function is expected (`E3804`); a ref is assignable to a
    literal union over it. `pairs:` elements follow WIRE §4.1 (a translated `export fn` has a
    non-finite parameter: not `Bool`, an enum or a ref into a table). `E1903` containment is
    transitive (a record holding the input record through fields counts) and covers every type
    written in the package, bodies, aliases and signatures included; paths are counted once per
    record, a cycle reaching the record counting as many. Keyed-list keys written twice (literal
    elements, `entry t.k`) are `E3102` statically, the `DECISIONS 155` way; an entry key is an
    `INT` only for an integer key field, else `E3002`. An asset root is any path of WIRE §2.1,
    file-relative included; an unknown `@root` is `E7003`, as `project` words it. `load.text`
    needs a `String` type, `load.csv` without `header: true` needs `[[String]]` (`E7116`). An
    interpolated emit string is `E1132`, which the parser does not reach there. A `Name`
    argument is qualified only for another package (ERRORS §1.3); a static `E3201`/`E3204`/
    `E3205` relates the field or let it is stored in (EVALUATION §4.3); `E2006` writes the project
    directory as `.`. A package qualifier has no `Types` entry (DECISIONS 152).

173. **`wire` decodes, `load` reads.** `wire.Decoder{Bag, Pkg, Host, Partial, Coll}` has
    `Decode(ctx, Selection, t)` (a JSON value, or what an `at:` path with `*` selected, each `*`
    level a map or list of the items below it), `Dir(ctx, []File, t)` (load.dir: the files'
    selections and stems) and `CSV(ctx, header, rows, t)` (cells already split by RFC 4180; a nil
    header is `[[String]]`). Each returns the value, `ok` (false after any finding: the value is
    poisoned, WIRE §3.4) and a Go error for a misuse the checker or `load` should have refused: a
    type with no wire form met while decoding (`Range`, a function, `Kind(V)`, a case type), a `*`
    level whose type is no map or list (`ErrStar`), a default or dereference without a host.
    `wire.Host` is `Default(ctx, f, Instance, via)` (defaults are expressions, which only the
    evaluator runs, `= none` included, so `wire` imports no `syntax`; only an optional field
    without default needs no host; no default is evaluated once the decode has failed) and
    `Deref(ctx, ref)` (a discriminant read through a ref); the host reports its own false
    results. Files,
    globs, `at:` (`E7106`), CSV syntax (`E7113`), encodings (`E7105`) and `W7107` stay in `load`.
    Reason: one mapping, the file forms around it in the package that owns them (§3).

174. **What decoding reports, and where.** Every finding carries the value's RFC 6901 pointer (a
    key's finding: its member value's) and no value path, which a keyed-list element's key,
    decoded after the failing field, would need. Besides WIRE's codes it reports the TYPES codes
    the mapping meets: `E3201` (integer, code key or Duration out of range, a token past `Int`
    quoted as written), `E3202` (a float overflowing its width), `E3301`, `E3302` (at the object;
    for CSV at the header row for a missing column, at the cell for an empty one), `E3312`,
    `E3102` (two load.dir files with one stem at `File.At`, two `$id` cells at the second),
    `E7104` (a repeated CSV column), `E7116` (a CSV column naming a field no cell can hold,
    `load.csv`). Keyed-list keys (`E3102`), refs (`E3501`), retired members and cases (`E3506`)
    and assets stay stage B's (DECISIONS 146). `E7110`'s `int`, `bits` and `retired` variants
    quote a scalar's JSON; a container there, and a `null` path intermediate, get the `kind`
    variant, whose type for an intermediate is its record's. `E7111`'s hint is the wire value of
    the member whose Canon name the text is, else the wire value chosen by `E2102`'s edit
    distance rule (DECISIONS 153). A bitmask past `Int` has bits no code has (codes are at most
    2^62): `E7111`'s `member` variant quoting the token. When an inline variant's tag fails, the
    keys of every case (fields, pairs slots, `$` keys) are claimed: no `E3301` follows from the
    one mistake (EVALUATION §7.2). Two object keys
    decoding to one key (`0` and `-0`, the only case) are `E3317` quoting both as written. A table
    key or stem is a `WORD` (`^[A-Za-z_][A-Za-z0-9_]*$`, not `_`), as table literals accept
    (TYPES §9.3); a root table, like a root record, ignores `$schema` (§5.12), a root map does
    not (its keys are data). A whole decoded table or keyed list is `Decoder.Coll`; a field collection some
    ref of the type targets is that interned collection, its owner the enclosing instance, as a
    level-1 ref's (RES-03); any other gets a collection of its own. Numbers are exact decimals
    whose exponent saturates, never `big.Rat` on a token (`1e999999999` would exhaust memory).
    Reason: WIRE §3.4's "every mismatch", with the codes TYPES already owns.

175. **A dependent value on a `Never` branch decodes as a symbol; verification rejects it.**
    WIRE §5.9 and §9 give `E3802` to a present value there, ERRORS gives `E3802` to `verify`, and
    TYPES §11.6 resolves dependent values in verification. The decoder computes every branch
    (reading discriminants through `Host.Deref`) and decodes against it; on `Never` it keeps the
    value as a `value.Symbol` (a string's text, else its compact JSON), which stage B rejects as
    a name the computed type lacks. Gap: `verify` does not evaluate dependent types yet
    (DECISIONS 147), so until it does such a value is not reported; `E3801` likewise waits.
    Reason: one owner per code, and a decoded value is what stage B verifies.

176. **CSV cells (WIRE §6.6).** An integer cell is a Canon `INT` (underscores between digits, no
    leading zero, `0x`, `0b`) with an optional `-`; a float cell an `INT` or `FLOAT`; a Duration
    cell either, exact, in the field's unit; a code cell or integer ref key `-?(0|[1-9][0-9]*)`.
    An unknown wire value is `E7111` and a range `E3201`/`E3202`/`E3203` as in JSON; only a cell
    that is no literal of its type is `E7108`. A cell equal to a string `none:` marker's text, or
    to another marker's JSON, is `none`. A column names a field by its one-key wire name; path,
    inline and pairs fields have none (a column naming one is `E3301`). A `table R` without a
    `$id` column is `E3302` naming `$id` at the header, and a `$id` value used twice `E3102` at
    the second cell naming the first, as two load.dir stems (gaps: §6.6 names neither). Reason:
    the strictest reading of each row of the cell table.

177. **Checker details the re-reviews settled.** The extensions of `asset(…, ext: [dds, png])`
    written as identifiers name no object, like an import path's leading parts (DECISIONS 151,
    152): `Symbols` holds only `IdentExpr`s. A WORD key of `entry t.k` is a member of an enum key
    field (`E3003` otherwise), a key of a ref key field (checked by `verify`), a `String`, or
    `E3002` (`expected Int, found String`). A name no scope has, checked against the error type,
    is silent (TYPES §1): it may be a contextual name of the type that failed. A join with an
    erroneous branch is the error type with no finding. `{ ...x }` with no expected type and `x`
    not a record is `E3305`. In `p == q` with `p: P(*)`, a bare `q` is read against `P(*)` and
    so stays symbolic even when a field `q` is in scope (§4.1 step 1 comes before scope; gap
    for Louis: §11.4 may mean the field).

178. **A multiline string keeps its value over FORMATTER §2.** Re-basing (§3) replaces the old
    prefix of each content line; a content line of blanks longer than the prefix keeps what follows
    it, and trailing blanks of content lines stay, since GRAMMAR §2.6 makes them part of the value
    and §1 forbids changing it. So a line inside a multiline string may end with a blank, against
    §2's "no line ends with a space or a tab". Gap for Louis: §2 could except string content.

179. **A brace-list item starting with `.` keeps a comma before it.** In a broken list (FORMATTER
    §6.1: no commas, one item per line) a line starting with `.`, such as a shorthand lambda key in
    `{ 0: 0, .a: [] }` or a `search` item, continues the line before (GRAMMAR §3.1 rule 3), and §10
    forbids adding parentheses; the item before it ends with `,`, a separator run, written right
    after that item and before its trailing comments (as in `( )` lists). Found by fuzzing.
    Gap for Louis: §6.1 could state this exception.

180. **Baked `gen/go` also refuses refs into `load.defines` tables, and fields of a case without
    fields (extends 124).** CODEGEN §5.8 gives such a ref a key getter plus `XxxValue() int64`
    and a baked sorted `(name, value)` table of the defines the emit's refs use; neither exists
    yet, and a key getter alone would ship half the API. So any ref whose target is a define table
    (a field, a list element, a map key, a lookup parameter) and any package whose IR carries
    `Defines` is `ErrUnsupported` naming the table; the define value getter and table land with
    `load` (M2/M3). A field, element or result typed as a case without fields is refused too: §5.5
    gives that case no Go type. An optional `Never?` field is omitted (§4.4); a plain `Never`
    reaches the generator only if stage E missed its `E8012`, and is refused. Messages name kinds
    (`Never`, `ref`), never their numbers.

181. **A Float32 -0.0 is `float32(math.Copysign(0, -1))` (amends 123).** `math.Copysign` is a
    `float64`, which a `float32` field, element or value does not accept; the conversion keeps
    the sign. A -0.0 constant stays refused.

182. **Go names: imported packages are escaped like the standard ones; overrides are
    exported.** CODEGEN §3.4 escapes, in lowercase positions, the package names generated files
    import; the Go package names of imported Canon packages are imports too, so a lookup
    parameter `q` next to `import q` is `q_` (`fn f(q: q.Q)` is `func F(q_ q.Q)`), and the
    builder's baked-data local `d` is `d_` when a package named `d` is imported. An escaped name
    that is itself an import (`q` and `q_` both imported) is `ErrNameCollision`. A `@go(name:)`
    override names public API (§1.3: exported identifiers), so a non-exported one is `ErrName`.
    A struct's storage and its methods are one Go selector namespace, checked as one scope.

183. **Reference layout: lookup tables built from the baked data return `*[N]T`, and
    generated Go is `gofmt -s` clean (amends 122).** `sync.OnceValue(func() *[6]*Column {…})`
    is indexed through the pointer (`columnOfTable()[s]`), so no call copies the table
    (`GroupedAreasVisibleTo`: 3.9 to 2.0 ns/op); a one-cell precomputed fn keeps its value. An
    element of a slice or array literal omits its type (`{…}` for `&T{…}` or `T{…}`), as
    `gofmt -s` writes it; the compile tests check that `gofmt -s -l` lists nothing.

184. **`pairs:` keys are checked by template, never slot by slot (WIRE §4.1–§4.2, SPEC §1).** A
    template whose keys start with `$`, or whose keys meet the other template of the same field,
    is `E3316` `pairsTemplate` (the rule of §4.1's `pairs:` row); a key of another field is
    `collision` or `prefix`. A clashing template stays compared, so later fields still meet it.

185. **The evaluator's cost model and recovery choices (EVALUATION §7, §12, §13).** Every AST value
    node evaluated costs one step, a callee's name and a method's `.m` included, so `xs.len()`
    is name + `.len` + call + the built-in's 1 (GRAMMAR's postfix ops, §12.1's shorthand `1 + 2`);
    a string template is one node plus its interpolations; `in` costs its node only; `sortBy`
    charges comparisons only, and every other built-in charges per element as it takes it,
    before invoking its function; `contains` by key costs the entries scanned. The budget's last
    step is the one that makes the count reach it (`steps >= budget`: E4401 there). A value
    completed while others are being forced is verified once none is (a ref may point into one
    still being built); stack frames name the fn (`Record.m` for a method) at its call site,
    lambdas counting toward the depth but not listed. Steps are charged to a check's name, else
    its record's or variant's, else `check`. Conversion findings carry the value path along a
    let's own literal nesting only; `E3503` keeps the unconverted record (marked invalid); a map
    key given twice at evaluation (`E3322`) keeps the first entry and marks the map invalid; a
    broken `entry` declaration poisons its let without a finding; retired members (`E3506`) and
    assets are left to stage B, which knows the retired-entry scope. Reason: one deterministic
    count, and no finding that another one already implies.
186. **The evaluator's surfaces beyond IMPLEMENTATION-PLAN §4.8 (DECISIONS 148, Louis-call 7).**
    `eval` may not import `rules` or `verify` (§3), so `Evaluator.Run` returns `eval.CheckRun`
    (rules.Run's fields) and `Evaluator.Test(ctx, test, Builder)` takes a `Builder` (verify + the
    instance checks of a subject, findings returned) that `build`/`rules` implements: one
    adapter each, a field copy. `Where` finds a predicate's file by its expression (types.Predicate
    has no package). `host.Verify(…) bool` stays frozen: the build adapter calls
    `verify.Check`, then `ev.Poison(root)` on `Result.Poisoned` and `ev.ReportUnbound(root, ref,
    path)` (E3505, eval's) for each `Result.Unbound`, and returns `Result.Valid`. `Call` names
    its outermost frame `canTransition(open, taken)`: EVALUATION §2.3's "while computing" is
    English no code or variant carries (gap). A broken test does not run (`TestRun.Broken`); a
    hard error outside an expect subject stops it and is reported. Reason: the frozen contract
    kept, every new surface one adapter away from its consumer.
187. **Const cycles and layers (DECISIONS 172, EVALUATION §3.2, §9.3).** A broken const, when
    forced or met by a fold, has its initializer uses followed to a cycle through itself, which
    is reported from its first const in (file path, position) order at the use closing it, so
    every const of it and the folder report the same `E4301` once (§14). Layers: an amendment
    replaces copy on write; the amended field becomes `Set`, and only the later defaulted fields
    of the instance whose field it replaced are re-evaluated; a path through a missing key,
    index or `none` is `E1905`, hard. An amendment adding an entry to a stable table, or changing
    a `@stable` field, is not applied: its let is poisoned and the amendment listed in
    `Evaluator.StableAmendments()` for `lock` to report `E6004` (lock's code; its txtar belongs
    in `internal/lock`, which EVL may not edit: gap until lock or build reports it). Reason: no
    lenient amendment, and one owner per code.

## 2026-09-24 — Louis

188. **`canon infer` is dropped from v0.1.** It sees only observed JSON values: it reads neither
    the JSON Schemas' constraints nor the loaders' clamps and defaults, so its draft is rewritten
    by hand anyway, and it is a one-off tool. Each domain's first type is written by an agent that
    reads the schema, the loader and the data together, then proven by `canon check` over every
    file. `canon convert` stays. This supersedes CLI.md §3.9 and step 1 of §6.4, and the `infer`
    rows and §8.2 of IMPLEMENTATION-PLAN; the `internal/infer` package is not built.

189. **The sovcommon `teamboard` integration leaves M1's acceptance.** IMPLEMENTATION-PLAN §6 M1
    item 6 (replace teamboard's hand-written loader in a sovcommon branch) is postponed until
    Louis schedules it; M1 is accepted on items 1–5. It stays a handoff in `meta/handoff/`.

190. **The GEN-01 review is the orchestrator's, not Louis's.** At the start of M2 the compiler
    regenerates `examples/pipeline/expected/`; the orchestrator reviews the diff against the
    conventions of the existing Go and C++ code (sovcommon, Source), decides, and M2 continues.
    The diff and the choices made are listed in `meta/handoff/` for Louis to read when he likes.

191. **The M4 studio integration spike is postponed.** IMPLEMENTATION-PLAN §6 M4 item 7 is dropped
    from M4's acceptance: the current resourcestudio will not be adapted, a new studio will be
    built from scratch on the Canon API. M4 is accepted on its other items.

192. **A generated file's header is one or two lines, in every language** (Louis, given live;
    the wording below is the orchestrator's reading, to confirm). Line 1 is the CODEGEN §2.4
    marker, unchanged. At most one more line follows: in Go's main file,
    `// Package <gopkg> is generated by canon from package <canon.pkg>.` (after a blank line, as
    Go requires); in a conformance file, one line `// Conformance vectors of package <pkg>,
    computed by the Canon evaluator.`; nothing in the other files. The package doc (GRM-08 `///`
    lines) and T1 leave the file header; T7 shrinks to that one line. This amends CODEGEN §2.5
    and SPEC §156 ("becomes the Go package doc and the C++/TypeScript file header comment").
    The declaration-level templates (T2–T6, T8–T11) are not headers and stay. The generated code
    follows the conventions of the code beside it: Go those of sovcommon, C++ those of Source's
    recent code, never its legacy style (no `m_` members, no `C` class prefix).

200. **Generated-program testing joins the test strategy (Louis, 2026-09-24; amends
    IMPLEMENTATION-PLAN §7.7).** Right after M1, QA builds a program generator in
    `internal/testkit` and four nightly property suites, each under a memory cap:
    rule-targeted mutation of the examples (one operator per ERRORS.md rule, the exact code
    expected); grammar-driven generation with token mutations (valid programs parse and survive
    format-reparse unchanged; corrupted ones get a located finding, never a panic);
    type-directed well-typed programs (check clean ⇒ build succeeds, generated Go compiles, its
    answers equal the evaluator's); metamorphic variants (renames, reordering, comments and
    whitespace change no finding and no output). A counterexample is shrunk and kept as a txtar.
    Reason: every rule is proven where someone thought to test it; this proves it where nobody
    did.

## Decided without Louis (autonomous session 2026-09-24), to review

193. **Which sovcommon conventions baked Go adopts (DECISIONS 192, the gen/go review).** Adopted:
    the id type and member are `<Element>ID`, `<Element>ID<Key>` (GoCap on the suffix, as §3.2's
    list and decision 121's `GetInitialStatusID` already do; amends CODEGEN §3.3's `StatusId`);
    `FindBy<F>` reads a `map[<type>]int` index built once, never a scan (sovcommon's slice + map);
    the user package doc (GRM-08) and T1 are not generated (DECISIONS 192), and the Go runtime
    `rt.go` keeps its marker plus one line `// Package rt is the runtime of the Go code canon
    generates.` (amends §6.3's three-line doc). A case's `@go(name:)` replaces every name derived
    from the case (the type, `As<Name>`, the kind member), as a field's override drives its key
    getter (§3.5). A retired case's kind member gets `Retired.` like an enum member (§5.2).
    Kept, because a Louis decision or a collision rule requires them: unexported fields and
    getters (DECISIONS 4), the `Get` prefix (ACCEPTED-CHOICES 18: user fields never collide with
    `ID`, `Kind`, `String`), the `self` receiver (§3.4: reserved, so it never collides with a
    parameter; linters skip generated files), explicit enum values (`@codes`, stable wire), and
    interior-`_` storage names (decision 121). Reason: sovcommon's form wherever it is safe, the
    spec's wherever sovcommon's could collide. Queued for Louis in `meta/handoff/`.

194. **Stage E details the IR review settled (CODEGEN §4.4, §5.6, §12; EVALUATION §2.3).** An
    explicit `values: []` is expanded to every public value, as the backends read it (127). A
    define table or record is `E8012`/`E8151` for an emit whose target cannot represent it
    (baked Go, 180) or that needs its fingerprint (`emit json`, `data`/`embedded` modes: canon-fp
    has no Define form, 126), and a ref into a define table is `E8012` for baked Go (180), so
    `check` fails wherever `build` would (37). `E8005` covers every name gen/go declares, read
    from one name plan in `ir` that gen/go uses; `E8011` refuses an unexported Go override (182). A wildcard `_ =>` arm's branch
    is named after the first member it covers, in declaration order (§5.6 names only patterns).
    Every receiver and cell of an `export fn` is evaluated and each failure reported; nothing
    stops at the first. A foreign type's methods are precomputed on the importer's receivers,
    because its encoded `$` keys need them (WIRE §5.11, 128), beyond §2.3's own-package wording.
    `E8101` skips TS `types` mode (it emits no values); its other cases land with M6. Reason: no
    finding that `build` could meet and `check` could not, and no error hidden behind another.

195. **The evaluator's resource bounds and details the eval review settled (EVALUATION §3.3,
    §12; STDLIB §4.2).** `E4402`'s 10 000 frames count every live user frame of the invocation,
    across nested roots (forced lets, check runs, `where` re-runs, precomputations), so chained
    forcing is bounded. `+` on lists and strings charges one step per element or byte of its
    result (amends §4.2's "0 extra"), so doubling a value runs into `E4401`, never out of memory.
    Every walk over a value (equality, text form, set hashing) is iterative or charged per node
    visited, and graph built-ins run on an explicit stack; no input reaches the host stack limit.
    A set coerces its elements to the call's static element type before hashing (TYPES §7.5).
    A free `where` re-run past its cap (the budget) aborts that re-run only: poisoned, no
    `E4401`, evaluation continues. `toMap` evaluates `keyF`, detects `E4502`, then `valF`. The
    heaviest root of §12.2 is the first charged on a tie. A stage-A dereference and stage B's
    verification of the same ref both report `E3501` (different locations, §14 keeps both).
    Internal errors surface as `ErrInternal`; `withIdentity` keeps the same instance. Values
    nested deeper than the host stack allows, and shared values whose tree is exponential
    (`l = [l, l]`) under the free verification walk, are bounded only by these walks being
    iterative and charged: a code or value-size limit needs ERRORS.md and value.go (Louis-call).

196. **What `build` does where the spec is silent (the build review).** An error in any loaded
    package, selected or imported, blocks code, data and lock; the imported package's error
    findings are then reported with the selection's (API R2 extended), so nothing is refused
    without its reason. A build with layers never writes `canon.lock` (EVALUATION §9.3, LOCK
    §6.1 over §5). `Revision()` hashes `<dir>/canon.lock` of every directory holding a source
    file, and of its ancestors, whatever the selection (API S3: it cannot know packages unparsed). Two outputs at one path are `E8152` unless both are a runtime helper
    file (WIRE §8.1's letter). `--adopt` takes only a C++ header (CODEGEN §2.4); a JSON output is
    `E8001` even when listed. A target without a generator yet (M1: cpp, ts, view) is refused
    before analysis, naming the emit (`build.ErrNoGenerator`); a `load` before M3 is
    `build.ErrLoad`: both are Go errors, exit 2, until their milestone. `build` writes its outputs
    and locks itself, all or nothing on a write error (IMPLEMENTATION-PLAN §3; a crash between
    renames can leave a mix). Layers reach the evaluator in
    M1 (187 implements them; supersedes 143's "not applied before M3"). Reason: nothing written
    that `check` would reject, and every refusal names its cause.

197. **Completing 195 (the eval re-review).** `r.len()` is `max(end − start, 0)`, `E4002` if
    open (STDLIB §10 amended: a range's elements are `start, start + 1, …` below `end`, §1.2, so
    `len() == 0` iff `isEmpty()`). Every built-in that produces a string charges one step per
    byte of its result, before building it: interpolation, `join`, `replace`, `lower`, `upper`,
    `trim*`, `String`, format specs (amends STDLIB §7, §9.1). `value.Equal` and `CanonText` are
    iterative with a same-instance shortcut (no signature change to value.go). `E4402`'s
    "(n more frames)" counts every live frame of the invocation, the forcing roots' included, as
    its limit does. `Evaluator.Test`'s `Builder.Build(ctx, v, capture *diag.Bag)` reports into
    the expect's capture bag (amends 186's "findings returned"), whose limit is lifted so that
    `fails "text"` sees every finding; an expect's operand text is capped like `VisitedUpTo`.
    The per-byte charge adds to each built-in's listed cost (1, n, values visited), never replaces
    it; `value` gains `TextLenUpTo` and `TextUpTo` (additions beside the frozen value.go).
    `split` charges one step per part and per byte of its parts. `==` and every equality the
    evaluator runs (`in`, `contains`, `indexOf`, map and set lookups) charge one step per
    composite pair visited through `value.EqualUpTo` (amends TYPES §7.5 and §12.1's one node), so
    comparing wide shared values runs into `E4401`; the pair memo starts past a named threshold.

198. **A fieldless case's doc goes on its kind member (CODEGEN §2.6, §5.2; the gen/go review).**
    A case's doc sits on its Go type; a case with no fields has no type, so its doc goes on the
    kind-enum member, followed by `Retired.` when retired. Other kind members carry only
    `Retired.`. An optional `@stable` field is refused for every table, emitted or not (LOCK
    `E6003`). Reason: no prose lost, none duplicated.

199. **No evaluator work is unbounded per step (the fourth eval review).** Equality charges one
    step per pair compared, scalar pairs included, so `x in xs`, `xs == ys` and `contains` over
    n elements cost n (amends DECISIONS 185's "`in` costs its node only" and 197's composite
    pairs). A map finds a key through a hash index, never a scan, so `m[k]` stays O(1) and a
    map equality is linear. The set hash walks at most a named number of nodes per element (equal
    values share that prefix; the charged equality decides). Binding level-1 refs is iterative,
    and the evaluator remembers, per node and owned-collection set, subtrees holding no unbound
    ref, so repeated literals over one shared value do not walk it again. A set operation over n
    elements costs O(n) plus the equality charges of genuine hash matches: a map hashes by its
    size and an order-free sum of its entry hashes, computed once per map. `@stable` comparisons while applying an amendment are
    free, as layer path resolution is (§12.1), and iterative. Equality charges add to a built-in's
    listed cost (`contains` over n costs its visits plus n pairs), as 197's per-byte charges do. Reason: every step of the budget
    buys a bounded amount of time and memory, so `E4401` is the only way evaluation runs long.

201. **`Build` and `canon build` where API.md and CLI.md are silent (the api/cli review).** An
    unknown `Target` in `BuildOptions.Targets` is refused with `ErrBadValue` (`*ValueError`,
    expected `go, cpp, ts, json or view`), never dropped; a writing `Build` holds the project's
    write lock (S9) and returns the revision read after its writes (S10). The build report: a lock
    is listed only when it gains lines; text lists changed outputs under their target, prefixed
    `stale ` under `--check`; JSON lists every output, `unchanged` included; the JSON summary is
    check's (`truncated` kept) followed by `written` and `stale`; `-q` prints errors only. Every
    printed path is a display path, write errors included. Example goldens are written by
    `internal/testkit/golden -update` (Check, then Build into the fixture roots), and
    `goldens-check` also fails on a written output MANIFEST does not list. A golden module's smoke
    test lives in `internal/testkit/golden/testdata/smoke/<example>/`, copied beside a temporary
    copy of the module by goldens-vet (expected/ holds only compiler output).

202. **Generated names that are not identifiers, and imports in the name plan (IR round 2).** A
    name derived without an override that is not a valid identifier in its target (a field `_1`
    whose Go getter would be `1`, or `__` giving an empty name) is `E8011` at the declaration,
    as an override would be (ERRORS.md's template names the override; the message names the
    derived name), so `check` refuses what `build` would (37). The Go name plan declares an
    import only when the generated code will use it (`math` only for a `-0.0` literal), so a
    Canon import named like a standard package collides only where gen/go really imports it.

203. **A Go override also renames its storage; one E8005 per cause (IR round 2 review).** A
    field's Go storage name derives from its effective name (the `@go(name:)` override when there
    is one, lowerCamel'd, interior-`_` forms of decision 121 unchanged), so the remedy CODEGEN
    §3.5 gives for a collision fixes it whole. Two items colliding in several derived names
    (getter and storage) give one `E8005`, at the getter. The Go name plan covers baked mode; a
    `data`/`embedded`/`types` go emit is checked only for its overrides (`E8011`) until M2 gives
    those modes their plan.

204. **Conformance vectors where CONFORMANCE.md is silent (the conform unit).** For a Float bound
    the neighbours are the adjacent representable values (`math.Nextafter`), and an exclusive
    upper bound `..b` contributes b's predecessor, b and its successor (§6.2's `b − 1` is for
    integers). To collect §6.1's calls, `build` runs the package's tests with a budget of their
    own, discards their findings and keeps the calls made before any stop (EVALUATION §1's "build
    does not run tests" means reports nothing). A value first forced inside a vector evaluation
    is charged to that vector's cap. A vector stopped by a poisoned read cannot happen in a build
    that emits (errors block, 196); `conform` returns `ErrNoOutcome` as an internal error. A
    translated fn reading a path with no candidate rule (an input field, a ref, a method of self)
    is refused at stage E with the E900x code whose meaning fits, never at build time as a Go
    error; if none fits, it stays a Louis-call.

## 2026-09-24 — Louis (afternoon)

205. **Generated Go targets the current Go only.** The Go 1.23 floor (CG-10, IMPLEMENTATION-PLAN
     §6 M1 item 4 and §7.8's "Go 1.23 and current") is dropped: generated Go is built, vetted and
     smoke-tested with the Go installed here. Reason: no consumer needs 1.23.

206. **No integration before the language is proven flawless on advanced tests.** The sovcommon
     `teamboard` integration (postponed by 189) and every other consumer integration wait until
     the generated-program suites of decision 200 run clean (every ERRORS.md rule mutated, well-
     typed programs agreeing between evaluator and generated code, metamorphic variants) and
     conformance is green. M1.5 is therefore pulled forward: it starts now, beside M2.

207. **The spec follows the decisions, and technical calls are the orchestrator's.** When a
     DECISIONS item overrides spec text, the orchestrator rewrites that text (spec/, SPEC.md,
     CLI.md, IMPLEMENTATION-PLAN) in a reviewed commit, without asking; builders still never edit
     the spec. A technical gap is decided by the orchestrator (strictest consistent reading) and
     logged in `meta/decisions/log-<date>.md`; only direction questions go to Louis.

208. **`E7109` names what went wrong, by variant (amends 162).** The span and pointer stay as 162
     says. The detail becomes three variants: `eof` (the source ends before or inside a value: `JSON syntax
     error: unexpected end of input`), `depth` (the opener past the nesting limit of WIRE.md §3.1:
     `nested deeper than {limit} levels`), and `char` (every other case: `unexpected character
     {char}`, `char` the offending character written as a JSON string, WIRE.md §7.3, a source
     excerpt and so `Text`). Reason: Louis's 2026-09-24 demo read `JSON syntax error: ""` as a bug
     (the gap 162 left for him); variants keep English inside `internal/diag` (DECISIONS 27).

209. **A broken check or field default breaks its record or variant (TYPES.md §1).** The members
     that run implicitly on every value of a record or case — its checks (named or not, `check`
     and `warn`, blocks included) and its field defaults — are part of its body: when one is
     broken, the record or variant is broken, and so is everything naming it (no data typed by it
     is decoded or verified; its static findings are all reported). A broken method breaks only
     the declarations that call it. Reason: a broken check reached the evaluator as an internal
     error (demo of 2026-09-24); skipping only that check would widen the frozen `check.Info`.

210. **Implicit frames: field defaults and `where` runs (amends 195, 197; EVALUATION.md §3.3).**
     Evaluating a field default and running a `where` predicate (the stage-B root included) each
     open one implicit frame that counts toward E4402's 10 000 limit, so no input reaches the host
     stack (195). Implicit frames are never listed in `Stack` and are not counted by `(n more
     frames)` (197's count is the listed kind); they cost no step (§12.1 already charges their
     nodes). An E4402 met on entering an implicit frame is reported at the default expression or
     at the predicate: the frame has no call syntax, and that is its only source text. A fold
     (TYPES.md §15) is constant-only: it reads no let (a key dereference reads one), calls no user
     fn or method and runs no `load`; one that does stops without a finding (150's E3015).
     Reason: three valid programs overflowed the host stack (overnight run, A3 eval).

211. **A broken brace list keeps a comma wherever GRAMMAR §3.1 would join the lines (extends
     179).** In a broken brace list (FORMATTER §6.1: no commas), the item before a line break ends
     with `,` when that item's last token is in rule 2's cannot-end set (the keywords `and or not
     in is else where as` used as data-symbol names, LEX-08(a)), or when the next item's first
     token is in the part of rule 3's continuation set that can start an item (`.` and the keywords
     `and or in is else where`); FORMATTER §6.1 states the exception. The line rules stay lexical:
     a keyword used as a data word joins like the keyword (this supersedes the M1.5 log call
     "keywords used as names … no join"). Blank lines between own-line comments follow FORMATTER
     §4 (removed inside expressions and `( )`/`[ ]` lists), except between two doc-comment blocks,
     which stay apart everywhere (§8.1: formatting never moves or merges a doc comment). Reason: an
     enum member named `in` reparsed as a continuation (E1116) or could not end a line (E1117);
     two `///` blocks merged (progen and fuzzing, overnight run).

212. **`fits` measures a group as the printer prints it (amends 170).** A group met while measuring
     is broken if it holds a hard line break, or its own written-broken bit is set, **and its holder
     is in BREAK mode**; under a FLAT holder it is flat (FORMATTER §7.1's print pseudo-code); a
     genuine hard line break still forces the holder itself (§8.2). Reason: `fits` stopped early on a
     group the printer later printed flat, and a parameter list overflowed (overnight run).

213. **Stage E and broken or refused input.** A broken declaration (TYPES.md §1) never enters the
     emit IR: no stage-E rule judges it, its names or a pair it is part of (an E8005 between a broken
     and a sound declaration is a cascade). An emit whose mode is refused (E8009) keeps its `out`,
     directory and package in every rule that does not depend on the mode (E8004, E8007, E8008,
     E8011 of its package name, E8104); only mode-dependent validation is skipped. A name derived
     from a refused `@go`/`@cpp` type name (its members, id type, case types) is that type's E8011,
     not its own; provenance decides, never a text prefix. The default Go `package` (the last
     element of `out`) is validated like a written one: not a Go identifier, or a keyword, is E8009
     `package` at the emit. Reason: A3 ir review (overnight run).

214. **Every syntax error breaks the declaration holding it; misplaced constructs are still
     checked.** TYPES.md §1 (a declaration with a static error is broken) holds for lexer and parser
     findings too, not only through a recovery node (IMPLEMENTATION-PLAN §4.7): the checker breaks
     every declaration whose span contains a syntax finding (E1101, E1109, E1129, E1130, E1134,
     E1135 …), and a broken declaration is never evaluated. A misplaced but well-formed construct
     (`return`/`expect`/`break` out of place, a brace literal, `if` or `match` as a header operand)
     keeps its normal node, so its operands are checked and their own findings reported (GRAMMAR
     §10, SPEC §10.3: every error is reported; 209). An invalid format spec is not attached to its
     interpolation (no E4503 cascade). Reason: A3 syntax review (overnight run).

215. **The default Go package is read from `out` as declared, never from a checkout path
     (amends 213).** "The last element of `out`" (CODEGEN §2.1) is taken after resolving `out`
     against the roots as written in `project.canon`: `--root` overrides and the absolute checkout
     path never change a package name (DOCTRINE §5). When that last element would be the project
     directory itself (`out` resolving to `.`), there is none: `package` must be written, else
     E8009 `package` with an empty value. check validates it through `project`'s own resolver (no
     second copy), and ir derives the same name the same way. A literal holding a lexer error
     (E1109, E1110, E1111 …) has the error type (TYPES §1): no fold, no static check on a made-up
     value. Reason: check C1 review (overnight run).

216. **A comma is kept where dropping it would change what a `///` comment documents (FORMATTER
     §1, §11 over 168).** In a broken list, a comma whose removal would attach a `///` comment to
     another item, merge two doc blocks, or turn a `///` written after code into a doc comment is
     not dropped: it stays on its own line, where it was, with its comments (`a` / `/// d1` / `,` /
     `b`; `a` / `, /// x` / `b`). Every other comma follows 168 and 211. Reason: FuzzFormat found
     `record A{A:A\n///\n,A:A}` changing W1001 (overnight run).

## Still open

See SPEC §23: the name, several views per type, binary layouts.
