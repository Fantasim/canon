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

## Still open

See SPEC §23: the name, several views per type, binary layouts.
