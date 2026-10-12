# ADR-0020 — API 0.1.6: overlays take a context, a per-call language, placed roots

Date: 2026-10-12. Status: accepted (orchestrator, under IMPLEMENTATION-PLAN §4's review rule:
`api` is a frozen contract; `spec-reviewer` reviewed the diff as the proxy of its consumers,
`cli`, `testkit`, `internal/lsp`, and the studio through the API; DECISIONS 344-346; handoff
2026-10-12-canon-next-versions F13, F16, F17).

## Context

- **F13.** API.md S11 promised a context on every method, while §3.4's `SetOverlay` and
  `ClearOverlay` took none. Both are writers (S9): an overlay call waited, uncancellable, behind an
  `Edit` or a writing `Build`. The language server avoided it by calling the workspace's
  ctx-taking writers directly. DECISIONS 285 kept the gap open.
- **F16.** `CheckWith` and `Evaluate` took a language per call; `Value` and `Edit` only had
  `Options.Lang`. An editor showing several languages opened one project per language (about
  2 GB and 10 s each on a large project) only to translate named-check messages.
- **F17.** An editor could not preview the assets of a root placed elsewhere on its machine
  (`--root`, `Options.Roots`, `project.local.canon`) without re-implementing root placement; the
  view model deliberately shows declared places only (108).

## Decision

```go
// 344, breaking:
-func (p *Project) SetOverlay(file string, content []byte) error
+func (p *Project) SetOverlay(ctx context.Context, file string, content []byte) error
-func (p *Project) ClearOverlay(file string) error
+func (p *Project) ClearOverlay(ctx context.Context, file string) error
// 345, additive:
+type ValueRequest struct { Path string; Lang string }
+func (p *Project) ValueWith(ctx context.Context, r ValueRequest) (*Value, error)
 type Edit struct { …; Lang string `json:"lang,omitempty"` }   // last field
// 346, additive:
 type ProjectInfo struct { Name, Doc string; Roots []RootInfo }
+type RootInfo struct { Name string; Dir string; Absent bool }
```

- S11 now excepts `Root`, `Close` and `Revision`, which never wait for the write lock.
- A per-call language re-renders named-check messages from the kept analysis
  (`internal/build/callang.go`: `ListIn`, `OwnIn`, `CauseIn`), which return exactly the old
  results for `""` and `Options.Lang`. Nothing is kept per language; counts and refusals do not
  change.
- `RootInfo.Dir` is the lexical placed directory (`project.Layout.RootDir`), not resolved through
  links; Canon never reads an asset for the caller (no `AssetFile`).
- Inside: `build.Project.Info` gave way to `build.Facts`; the workspace's ctx-less overlay
  wrappers are deleted (the `*Context` writers remain, so the language server is unchanged).

## Consequences

- 344 breaks callers of the two overlay methods in a patch release, against API.md §1.2's
  "minor release": no caller outside this repository used them (searched 2026-10-12), and the
  release notes name the break.
- `ProjectInfo` holds a slice, so `==` on it no longer compiles; a caller comparing it must use
  `reflect.DeepEqual` or compare fields. Named in the release notes.
- 345's localisation of a cause evaluation's findings is a forward guarantee: only hard findings
  poison a root today (EVALUATION §7.1) and compiler diagnostics have no translation key (I18N K8).
- The studio keys its projects by (layers, edit layer) only.
