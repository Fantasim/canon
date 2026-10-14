# ADR-0021 — v0.1.9, the robustness release: every decision, and how the run builds it

Date: 2026-10-14. Status: accepted. Scope: Louis, 2026-10-14 ([log-2026-10-14](log-2026-10-14.md),
"Louis's calls", "N11 corrected"). Designs: the orchestrator (technical calls are its own, DECISIONS
207; nothing technical goes to Louis), from read-only research over `main` at `6a5fc5f3` (tag
`v0.1.8` = `c33919a4`, plus one docs commit), Resource, Source and the Sovereign services. Amends
[ADR-0010](0010-edit-journal-recovery.md) (§2.2).

## 0. How to run this ADR

- **It is decision-complete.** The run builds what it says; it does not redesign. Each *Decision*
  binds; every "Spec text" block is final wording, applied verbatim by the spec sync (§7, S1).
- **DECISIONS numbers.** 374-382 are the next free numbers on `6a5fc5f3` (373 is the last). If one is
  taken when the run starts, shift every number of this ADR by the same offset and log it.
- **File:line references** are from `6a5fc5f3`. They locate; they do not bind.
- **Fallback, for a small question this ADR does not answer** (a real design question is a gap,
  below): (1) a spec document that answers it wins (DOCTRINE §2); (2) otherwise take the reading
  that refuses more, writes less and keeps today's behaviour for everything this ADR does not name
  (strictest option, DECISIONS 25); (3) log the question, the readings and the choice in
  `meta/decisions/log-<date>.md`; (4) never widen the scope: a new bug is logged as owed in
  `meta/state.md`, not fixed, unless it blocks an acceptance named here.
- **A real gap** (two readings that change behaviour, neither stricter) stops that unit only; the
  orchestrator decides it, logs both passages and the choice, and the rest of the run goes on.
- **Only a philosophy question goes to Louis**, in a file under `meta/handoff/`; the run continues
  on everything it does not block.

## 1. Context and scope

- Louis, 2026-10-14: "I don't want to do optimization if it's risking the robustness of Canon."
  v0.1.9 is the robustness release; speed work stays parked ([plan](../plan.md), "Later, deferred,
  passed and refused"). No item below may be "improved" with a cache, a parallel stage or a partial
  re-check.
- The scope is Louis's: the known bugs of `meta/state.md` "Owed, small", the refresh stat-trust fix,
  v0.1.4's owed cross-root trap and ConstantID clash, **N11** (urgent), **import tiers** (extremely
  important), M4 calls (a) O2 and (c) the view-model refresh target, and the release gates.
- `v0.1.8` is tagged and pushed (`c33919a4`); the run starts from `main`.
- Not in v0.1.9: M4 call (b), the `check.Info` layered maps, is v0.2 (plan.md, "v0.2"); everything the
  plan lists under v0.2 or under "Later, deferred, passed and refused" stays out.

| # | Item | § | DECISIONS | Units (§7) |
|---|---|---|---|---|
| 1 | A refresh trusts size + mtime | 2.1 | 374 | STAMP |
| 2 | `Recover` at Open takes no writer lock | 2.2 | 375 | J-WRITELOCK, J-EDIT, J-BUILD, J-WORKSPACE, J-CLI |
| 3 | Transient `ErrJournal` for two processes editing different packages | 2.2 | 375 | (same) |
| 4 | The asset kind test follows links unbounded | 2.3 | 376 | ASSETS |
| 5 | A case-only rename of an `out` directory is `E8001` | 2.4 | 377 | DIRCASE |
| 6 | `NoHook` filled for the import closure only | 2.5 | none: a documented limit | NOHOOK |
| 7 | A layer or new entry file cannot name another package's non-nameable member | 2.6 | 378 | Q-SYNTAX, Q-EDIT, Q-FORMAT, Q-CHECK, Q-LSP, Q-EDITORS, Q-PROGEN, GUIDE |
| 8 | v0.1.4's cross-root trap and ConstantID clash | 2.7 | none: closed by 347, 348 | none |
| 9 | N11, positions are not ids | 3 | 379 | N-EVAL, N-CHECK, N-IR-PLANS, N-IR-W8031, N-GO, N-CPP, N-TS, GUIDE |
| 10 | Import tiers | 4 | 380 | T-PROJECT, T-BUILD, T-EXAMPLES, GUIDE |
| 11 | M4 (a): the scan skips dot-prefixed files (O2) | 5 | 381 | O2 |
| 12 | M4 (c): a target for the studio's refresh after an edit | 6 | 382 | VMBENCH, §8 |
| 13 | Release | 8 | none | orchestrator |

## 2. The known bugs

### 2.1 A refresh compares the change time and the file identity (DECISIONS 374)

**Facts.**
- API.md S1 (`spec/API.md:198-201`) stats every file of the read set and every listed directory, and
  re-hashes "a file whose size or modification time changed".
- The one stat-based "changed?" decision is `ifStale` (`internal/workspace/refresh.go:26-33`), fed by
  `stampOf` (`snapfs.go:297-302`, called from `readFile` :229 and `readDir` :257). The stamp is
  `{class, size, mtime, zero}` (`snapfs.go:38-43`). A name read less than `racyWindow` = 2 s
  (`constants.go:6`) after its mtime is read again (`refresh.go:28`).
- A stamp mismatch is always followed by a content comparison (`refresh.go:73-85`): a false positive
  costs one read and hash, never a snapshot, revision, re-check or event.
- Everything else that judges change compares content: the build's re-read (`build/reread.go`,
  `workspace/fresh.go`), edit N9 (`edit/commit.go:207-228`), the load memo (`load/memo.go`).
  Nothing persists a stamp; no golden depends on one.
- A `Watch` does not close the gap: OS events only mark the hub dirty and the refresh still decides
  by stamp (`watch_hub.go:336-385`, `watch_os.go:85-98`). The LSP has no `Watch`; the studio uses
  `canon.Open` plus `Watch`, so it gets the fix.

**Problem.** A same-size file replaced with its mtime kept (`cp -p`, `rsync -t`, `tar x` of an
archive with fixed times, `touch -r`) stays stale in a long-running project (studio, LSP, `--watch`,
repeated API calls): reads answer from the old content, every `Edit` of the file fails N9 as
`*StaleError` (the next refresh is just as blind), a writing `Build` fails after its one retry, and
a file made readable by `chmod` stays unreadable (`chmod` moves only the change time). Directories
have the same hole: after `tar x`, `rsync -a` or `cp -a` an added or renamed source is missed.
One-shot CLI runs and Canon's own edits are not affected.

**Decision.**
1. The stamp gains `ctime` (nanoseconds), `ino` and `dev`; it keeps `size`, `mtime`, `class`, `zero`.
   "Unchanged" means every field equal; the new equality implies the old one, so nothing today's
   code catches is missed.
2. Directory entries use the same stamp.
3. The racy rule counts the change time: a name is read again while `at - mtime < 2 s` or
   `at - ctime < 2 s`.
4. The real fields come from `Stat`'s `Sys()` when it is `*syscall.Stat_t`, standard library only,
   build-tagged on writelock's platform set: `stamp_ctim.go` (`//go:build linux || openbsd ||
   dragonfly || illumos`; `st.Ctim.Nano()`, `st.Ino`, `uint64(st.Dev)`), `stamp_ctimespec.go`
   (`//go:build darwin || freebsd || netbsd`; `st.Ctimespec.Nano()`), `stamp_none.go`
   (`//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly || illumos)`; zeros).
   Android and iOS follow linux and darwin through Go's own tags.
5. Windows stays size + mtime: its stat has no change time and no file index, NTFS tunnelling restores
   the creation time of a name re-created within 15 s, a file index costs a handle per file per
   refresh (NFR-01), and the change-time API needs `unsafe`. S1 and 374 say so. Owed, not v0.1.9:
   re-reading the names an OS notification reports (§9).
6. The stamp stays in `internal/workspace` (IMPLEMENTATION-PLAN §3 gives it the refresh).
7. An FS whose `Sys()` is anything else (an embedder's FS, the in-memory test FSes) gets zeros. A
   change time of 0 never forces a re-read; the zero-ModTime rule is unchanged.
8. No CI change: one local cross-build in the acceptance instead.
9. Rejected: the inode alone (an in-place `cp -p` keeps it); the change time alone (a file prepared
   before our read and renamed over in the same tick); the birth time (not in Linux's `Stat_t`, does
   not move on in-place writes); re-hashing every call or a background re-hash (NFR-01);
   `os.SameFile` (it needs the old FileInfo kept, and on Windows it opens the current path).

**Code** (all `internal/workspace`).
```go
// snapfs.go
type stamp struct {
	size  int64
	mtime int64
	ctime int64  // change time; 0 where the stat has none (sysStamp)
	ino   uint64 // with dev, the file's identity; 0 likewise
	dev   uint64
	class class
	zero  bool // no modification time: the entry is read again at every refresh
}

// replaces modTime()
func (st stamp) racy(at time.Time) bool {
	return at.Sub(time.Unix(0, st.mtime)) < racyWindow || at.Sub(time.Unix(0, st.ctime)) < racyWindow
}

// stamp_ctim.go, stamp_ctimespec.go, stamp_none.go: one function each
func sysStamp(sys any) (ctime int64, ino, dev uint64)
```
- `stampOf` fills the three fields from `sysStamp(info.Sys())` (a nil or other `Sys()` gives zeros).
- `refresh.go:28` becomes `racy := st.class == classOK && e.stamp.racy(e.at)`; its comment and
  `constants.go:5`'s say "modification or change time".
- No `const` in a build-tagged file (the audit's const placement); one-line field comments.

**Spec text.**
- API.md S1 (replaces `spec/API.md:198-201`):
  > - **S1.** At the start of each call, unless a `Watch` is active, the project **refreshes**: it
  > stats every file of the current read set and every directory a glob or package scan listed, and
  > reads one again when its **stamp** changed. The stamp is the stat's size and modification time
  > and, where `Stat`'s `Sys()` is the platform's `*syscall.Stat_t` (Linux, macOS, FreeBSD, NetBSD,
  > OpenBSD, DragonFly, illumos), its change time, device and inode number, so a name replaced
  > keeping its size and modification time (`cp -p`, `rsync -t`, `tar x`, `touch -r`) or made
  > readable by `chmod` is read again. Elsewhere (Windows; an FS whose `Sys()` is anything else) the
  > stamp is size and modification time alone, and such a change is seen only once one of them
  > changes (DECISIONS 374). A name is also read again at every refresh while its modification time
  > is the zero time, or while it was read less than 2 s after its modification or change time,
  > since a later change within the same timestamp would not show. A changed hash or listing makes a
  > new snapshot; the same content keeps the snapshot. While a `Watch` is active, the watcher keeps
  > the snapshot fresh instead (§12).
- API.md W12 (`spec/API.md:1621`): "any change to the read set is seen" becomes "any change to the
  read set that changes a stamp (S1) is seen".
- DECISIONS:
  > 374. **A refresh compares the change time and the file identity (API.md S1, W12).** A file or
  > listed directory is read again when its stamp changed: size and modification time and, where
  > `Stat`'s `Sys()` is the platform's `*syscall.Stat_t` (Linux, macOS, FreeBSD, NetBSD, OpenBSD,
  > DragonFly, illumos, writelock's set; build-tagged, standard library only), its change time,
  > device and inode number. The 2 s racily-clean rule counts the change time as it counts the
  > modification time. Elsewhere, and for an FS whose `Sys()` is anything else, the stamp stays size
  > and modification time: Windows' stat has neither a change time nor a file index, NTFS tunnelling
  > restores the creation time of a name re-created within 15 s and an overwrite keeps it, a file
  > index costs a handle per file per refresh, and the change time needs an API `syscall` lacks. A
  > stamp that moves with the same content (`chmod`, a hard link, an extended attribute) costs one
  > read and hash, never a snapshot, revision or re-check; nothing is persisted. Reason: a same-size
  > file replaced with its modification time kept (`cp -p`, `rsync -t`, `tar x` of an archive with
  > fixed times, `touch -r`) stayed stale in a long-running project (the studio, the language server,
  > `--watch`, repeated API calls) until another change: reads answered from the old content and
  > every `Edit` of the file failed N9 as `*StaleError`; a file made readable by `chmod` stayed
  > unreadable. One-shot CLI runs and Canon's own edits were never affected (log-2026-10-14).
- IMPLEMENTATION-PLAN §8.5's planned cache stat table `(path, size, mtime, sha256)` is left alone
  (the cache is parked); §9 flags it for the cache's future ADR.

**Tests.**
- `TestRefreshChangeTime` (new `internal/workspace/stamp_internal_test.go`; cites API.md S1,
  DECISIONS 374). Skips unless `runtime.GOOS` is in a test variable {linux, android, darwin, ios,
  freebsd, netbsd, openbsd, dragonfly, illumos} (a broken build tag then fails instead of skipping).
  Per row: create the target under `filepath.EvalSymlinks(t.TempDir())`, `os.Chtimes` it to
  2020-01-01; `s := newSnapFS(project.OS(), nil, func() time.Time { return time.Now().Add(time.Hour) })`
  with `s.root` set (the late clock disables both racy clauses, so only the stamp is tested); read
  the target once; `tick`; apply the change; `Chtimes` back to 2020; `s.refresh` must report
  `changed == []name{{kind, abs}}` (the row's entry kind and the target's absolute path) with the new
  content in `next`. Rows: **in place** (`os.WriteFile`
  "aaaa" → "bbbb"); **renamed over** (a temp file, `Chtimes`'d, renamed onto the target); **listing**
  (`d/a.canon` renamed to `d/b.canon`, then `Chtimes(d)`: a directory entry); **made readable**
  (mode 000 → 0644, no `Chtimes`; skipped when `os.Geteuid() == 0`). `tick` sets the time of a
  probe file in a second temp dir until the probe's change time differs from the target's (5 s
  deadline), so the timestamp granularity never matters.
- `TestRefreshReplacedKeepingTime` (new `api/refresh_test.go`, package `canon_test`; same skip),
  subtests "in place" and "renamed over" (`c.canon.new` renamed onto the file): `writeFile`
  (`api/rename_fixture_test.go:27`) writes `project.canon` and `c/c.canon` =
  `"/// C.\npackage c\n\n/// N.\nlet n: Int = 1\n"`; `Chtimes` the source to 2020;
  `canon.Open(dir, canon.Options{})`; `Value(ctx, "c:n")` gives "1"; replace with `= 2` (same size),
  `Chtimes` back to 2020; `Value` gives "2".
- The in-memory tests `TestRefreshByStat`, `TestRefreshZeroModTime`, `TestRefreshRacilyClean`,
  `TestEditUnseenWrite` pass unchanged (their `Sys()` is nil).

**Acceptance.** Both new tests fail against the unchanged production files (prove it by copying the
changed files aside and back, never `git stash`) and pass with the change; `make check` green; one
local cross-build, `go build ./internal/workspace/` for GOOS/GOARCH in {linux/386, linux/arm,
linux/mips64, linux/arm64, darwin/amd64, darwin/arm64, freebsd/386, netbsd/arm, openbsd/amd64,
dragonfly/amd64, illumos/amd64, windows/amd64}; a failure inside a third-party dependency drops that
GOOS from the list (logged), a failure in workspace's own files is a bug.

**Unit STAMP.** `internal/workspace` plus the new `api/refresh_test.go`. Sonnet (small and pinned by
tests that fail today), opus review. About 45 lines of code, 150 of tests.

### 2.2 Writer locks decide an edit journal's liveness and reach (DECISIONS 375; amends ADR-0010)

**Facts.**
- ADR-0010:20: "One edit at a time. Any journal present blocks every `Commit` until it is recovered."
- A writer is alive by host and pid alone (`internal/edit/recover.go:29-32`; Unix signal 0,
  `workspace/alive_other.go:12`; Windows "the process can be opened", `alive_windows.go:8`). This
  process's own commits are found in an in-process registry keyed by the `b.Dir()` string
  (`workspace/process.go:14-58`); `api/session.go:22` makes it absolute with `filepath.Abs` only.
- `Recover` (`recover.go:35-105`) takes no lock. It runs from `api.Open` (`api/canon.go:95`) and from
  the commit's panic path (`workspace/edit.go:260`), nowhere else. Every CLI command opens through
  `openProject` (`cli/fail.go:98-109`) → `api.Open`, once per process.
- `Commit` runs `idle()` first (`commit.go:195-204`): any journal file gives `ErrJournal`, which the
  API reports as `ErrProject` (`api/edit_errors.go:42`).
- An edit locks `ExclusiveOf(Plan.Touched)` on the OS file system only (`workspace/writers.go:20-30`);
  `Touched` holds the owners of written files (`edit/apply.go:310-318`), and a JSON file's owner is
  the operation's package only (`edit/applysite.go:130-136`), while `canon fmt` locks every package
  that reads it (`cli/fmt_write.go:47-57`). The `canon.lock` change is appended after planning
  (`workspace/lockedit.go:33-50`).
- `internal/writelock`: `flock` per open file on Unix (`lock_flock.go`), share modes on Windows
  (`lock_windows.go`); no exported non-blocking acquire (only the internal `tryLock`); where
  `lockable()` is false (`lock_none.go:9`) `Acquire` succeeds taking nothing (`lock.go:72`);
  `doc.go` still describes the dropped `Held.Holds` re-plan.
- A writing build writes `canon.lock` (`build/emit.go:139-145`); no build reads a journal or rewrites
  a source. `canon fmt` writes under `writelock.Acquire` (`cli/fmt_write.go:22`). `canon new` creates
  `<pkg>/<last>.canon` with no lock (`cli/create.go:62-96`). `canon explain` opens through
  `sourceFS` (`cli/explain_source.go:22-26`), which embeds `build.WriteFS` but loses osFS's
  `lockWriters`, `SyncDir` and `Chmod`. The LSP never writes (`lsp/project.go:95`, read-only
  `bufferFS`). `--check` and `--only-root` builds write no package file and take no lock.

**Problems.**
- (a) A commits package `a` (its journal present from the journal write to the last rename) while B
  commits `b`: B's `idle()` fails `ErrJournal` after B planned and locked.
- (b) A died mid-commit, so its locks are free. A build or `canon fmt` holding `a` may write over A's
  torn state while a lock-free `Recover` rolls it back (lost updates, a mixed state), or the rollback
  later refuses and `Open` fails for good. Two `Open`s recover the same journal at once with the same
  stage names (one consumes the other's stage: an I/O error). Worst: recovery D judges a file "new",
  edit E commits after recovery B removed the journal, and D's late restore silently overwrites E.
- (c) In one process through a path alias (a symbolic link, a case or 8.3 alias), `Open`'s `Recover`
  counts its own live commit as dead and rolls it back mid-commit.
- (d) Journals left by v0.1.8 or earlier are judged by pid and block every commit.

**Decision.**
- **J1. A held lock is the liveness test** for a journal that records locks, on this host, on a file
  system with locks. A writer holds its locks from before writing its journal until after removing
  it, so a journal one of whose locks is free belongs to a writer that died or finished. Host and pid
  stay for journals that record no lock. Another host's journal is kept and never applied, as today.
- **J2. Journal format version 2.** A new field `"locks"`, right after `"pid"`: the sorted, unique
  package names (each passing `writelock.Valid`) whose writer locks the writer holds exclusively.
  Every commit writes version 2; it omits `locks` exactly when its `Site.Locks` is nil (J12). A
  journal without `locks` (version 1, or version 2 without the field) covers every package and is
  judged by host and pid. Version 1 is still read. A version-1 journal carrying `locks`, or a `locks`
  list that is not sorted unique valid names, or that is present but empty, refuses the whole journal
  (`ErrJournal`; kept).
- **J2a. Journals no other rule covers.** A journal that does not decode, or whose version is unknown,
  counts as recording no lock: `Meeting` and `Settle` skip it, `blocked` (J6) refuses on it, and
  `Recover` removes an undecodable one as today. A site with no Locker (`Site.Locks` nil, J12) never
  applies a journal that records locks: its `Recover` leaves it, and it blocks every commit of that
  site (fail closed; the next lock-capable `Open` recovers it).
- **J3. `Recover` at Open** takes journals in name order, one at a time. For a journal with locks it
  calls `TryAcquire` on all of them, exclusively, without waiting, all or none. Taken: it reads the
  journal again under them (gone: skip it; its `locks` changed: leave it), rolls back, removes the
  journal, releases. One held: it leaves the journal, and `Open` succeeds. `Open` fails only for
  journals it must keep, as today; an I/O error taking a lock is returned as is, naming the journal.
  `Open` never waits for a lock (it has no context, S11, and 319/327 refuse a timeout).
- **J4. Every writer that takes locks runs one protocol.** It acquires its own set (sorted, each
  package in its mode, waiting with its context, as today: a writing build takes its selected
  packages exclusive and the unselected packages it loads shared, `build/writers.go:39-43`).
  `edit.Meeting` then computes the transitive closure of this host's lock-recording journals whose
  `locks` meet the packages the writer holds in either mode. Every package such a journal records
  must be held **exclusive**: a package held shared, or not held, counts as missing. If one is
  missing, the writer releases everything and acquires the union (its own requests, every journal
  package upgraded to exclusive) in name order; it repeats until nothing is missing (the set only
  grows: the loop ends and cannot deadlock). It never acquires a lock while holding one (`flock` is
  per open file: the process would wait on itself). Then `edit.Settle` rolls back, in name order, the
  journals all of whose packages it holds exclusive (`Site.Held` lists only the exclusive ones), and
  removes them.
- **J5. An edit whose `Settle` changed a file returns `*StaleError` naming those files and writes
  nothing**: its plan and E19 re-check saw the torn state. The caller retries as for any stale edit.
- **J6. What blocks a commit.** A journal whose locks are disjoint from the commit's is ignored.
  `ErrJournal` (reported as `ErrProject`) for: another host's journal; a journal recording no lock; a
  meeting journal still present after `Settle` (fail closed); a journal at the commit's own name
  (checked before writing it). On a file system without locks any journal blocks, as today.
- **J7. An edit's lock set covers every file it writes:** `Plan.Touched`, plus the package of each
  written `canon.lock` (its directory, LOCK.md §2.1), plus every package that reads a written JSON
  file: the packages whose `Analysis.Reads(pkg)` (`internal/build/reads.go:181`), in the edit's
  pre-edit analysis, holds that file (not `JSONSources`, which re-checks and forces loads). All of
  them exclusive. A new `Plan.Unowned` lists written files
  with no owning package; when it is non-empty the commit runs lockless (`Site.Locks` nil, a journal
  without `locks`, refused by any journal): today's behaviour.
- **J8. The panic path** recovers only its own journal, under the locks it holds (through `Settle`);
  it no longer scans every journal by pid.
- **J9. The in-process registry** stays, for journals that record no lock only.
- **J10. Every other writer settles too**, so none writes over a dead edit's torn state:
  - *Build* (`canon build`, `--watch` reruns, API `Build`): a settle hook on `build.Project` (`build`
    cannot import `edit`), copied by `Over` (`build/reads.go:67`; snapshots are `tmpl.Over(fs)`,
    `workspace/project.go:56,63`), installed by `workspace.NewWith` through `build.WithSettle`, run in
    `run.lockWriters` after the acquire loop of J4. When it changed a file, the build's existing
    re-read (352) redoes the run.
  - *`canon fmt`*: the same loop in `fmtRun.write()` (`cli/fmt_write.go`) right after `Acquire`, with
    the layout from `build.Open(r.fsys, r.root, …)` as `fmt_json.go:21` does; its existing
    `unchanged` check then refuses a file changed since read.
  - *`canon new`*: takes the new package's exclusive lock and settles around the create (`runNew`).
  - *`canon explain`*: opens through a new `build.OSReading(onRead func(name string, data []byte))
    WriteFS`, an osFS with a read hook that keeps every capability (locks, `SyncDir`, `Chmod`),
    instead of today's `sourceFS` embedding.
  - Unchanged: `--check`/`--only-root` builds (no package file written, no lock), `check`, `test`,
    `refs`, `lock check` (they write nothing; their `Open` recovers with try-locks), the LSP (never
    writes, never recovers), overlays, embedders passing `Options.FS` (lock-free, 352: the pid rule,
    any journal blocks). Every settle ignores other hosts' journals and lockless journals, as today.
- **J11. One `Warn` line, O5's message, per `Settle` that removed a decodable journal.**
  `workspace.NewWith(b, logger)`; `workspace.New(b)` stays as `NewWith(b, nil)` (the LSP is
  untouched); `api/session.go:35` `workspace.New(p.b)` becomes `workspace.NewWith(p.b, p.logger)`.
- **J12. One helper decides lockability:** `workspace.lockerFor(fsys)` returns a non-nil
  `edit.Locker` only when `build.LocksWriters(fsys) && writelock.Lockable()`. `edit` and `build`
  cannot import `workspace`: workspace passes the Locker to edit (`Site.Locks`) and wires it into the
  `build.Settler` it installs; `canon fmt` and `canon new` reach it through
  `workspace.LockAndSettle`. `writelock.TryAcquire` returns `ErrNoLocks` where `lockable()` is false, never a vacuous
  success; `Acquire` keeps its vacuous success (352: the lock is advisory), and no liveness is ever
  inferred from it.
- **Rejected:** one journal per package (a multi-package edit, a rename across importers, needs
  atomicity across journals, and still needs locks for liveness); the status quo plus a bounded retry
  (a timeout, 319/327; leaves (b), (c) and pid liveness); pid and lock together (two sources of
  truth); waiting in `Recover` (no context at `Open`); refusing a commit on a meeting journal
  (long-lived processes stay wedged); recovering under only the intersecting locks (a build of the
  journal's other packages may be live); staying at version 1 with an optional field (an old binary
  would apply the journal without its locks).

**Interfaces** (exact; none is a frozen contract):
```go
// internal/writelock
func TryAcquire(dir string, reqs []Request) (*Held, bool, error) // all or none, never waits
func Lockable() bool
func Valid(pkg string) error // writelock's package-name rule
var ErrNoLocks = errors.New(...) // errors.go; returned by TryAcquire where lockable() is false

// internal/edit
type Locker interface {
	// TryLock takes every package exclusive, all or none, never waiting.
	TryLock(pkgs []string) (release func() error, ok bool, err error)
}
// Site gains: Locks Locker (nil: no locks) and Held []string (the packages held exclusive).
// Meeting: holding is every package held in either mode; need is every package of the closure
// of meeting journals (J4), to be held exclusive.
func Meeting(s Site, holding []string) (need []string, err error)
func Settle(s Site, logger *slog.Logger) (changed []string, err error)
// commit.go: idle() is replaced by blocked() (J6); Plan gains Unowned []string (J7).

// internal/build: a third downward seam (IMPLEMENTATION-PLAN §3), installed by workspace
type Settler interface {
	// Meeting returns the requests to hold instead of held (journal packages exclusive);
	// it returns held unchanged when nothing is missing.
	Meeting(held []writelock.Request) ([]writelock.Request, error)
	Settle(held []writelock.Request) (changed []string, err error)
}
func (p *Project) WithSettle(s Settler) *Project // Over keeps it
func OSReading(onRead func(name string, data []byte)) WriteFS

// internal/workspace
// lockerFor takes the file system type build.LocksWriters takes; nil unless J12 holds.
func lockerFor(fsys build.WriteFS) edit.Locker
// NewWith installs WithSettle; New(b) is NewWith(b, nil). Same result type as New.
func NewWith(b *build.Project, logger *slog.Logger)
// LockAndSettle runs J4 for canon fmt and canon new.
func LockAndSettle(ctx context.Context, b *build.Project, reqs []writelock.Request, logger *slog.Logger) (release func() error, changed []string, err error)
```

**Spec text.**
- ADR-0010: its status line becomes "accepted (own-process liveness refined with U5b; one edit at a
  time and liveness amended by ADR-0021, v0.1.9)", and this section is appended:
  > ## Amendment (2026-10-14, v0.1.9; ADR-0021, DECISIONS 375)
  >
  > Writer locks (352) replace "One edit at a time" and, where they exist, the pid test. A journal
  > (version 2) records the packages whose locks its writer holds from before writing it until after
  > removing it. On this host it is its writer's while one of those locks is held: `Recover` takes
  > them all without waiting before rolling it back and leaves it otherwise; a writer that holds one
  > of them first recovers it, holding all of them. A journal whose locks do not meet a commit's does
  > not block it. A journal recording none (version 1, an embedder's FS, a platform without the
  > primitive), or one from another host, keeps every rule above.
- API.md N10: replace "and the writer's host and process id" with "the writer's host and process id;
  and the packages whose writer locks it holds (DECISIONS 352, 375), none where no lock is taken".
- API.md N11, the liveness bullet, new text:
  > it keeps, and never applies automatically, a journal written on another host (O5). On this host
  > it rolls back a journal recording writer locks only holding all of them exclusively: `Recover`
  > takes them without waiting, all or none, and leaves the journal alone when one is held; a writer
  > already holding them recovers it under them. A journal recording none is left alone while its
  > writer runs on this host; one written by this process counts as running only while one of its
  > commits runs there, so `Close` then `Open` recovers one a failed rollback left;
- API.md N11, a new bullet: "it refuses whole a journal whose lock list is not sorted unique package
  names, or a version-1 journal with one;"
- API.md N11, the sentence "While any journal is present, every `Edit` that writes is refused …" is
  replaced by:
  > An `Edit` that writes, once it holds its writer locks, rolls back every journal of this host whose
  > locks meet its own, holding all that journal's locks exclusively (it frees its own and takes the
  > union in name order until no further journal meets them); if that changed a file, the edit is
  > `*StaleError` naming them and writes nothing. A journal whose locks meet none of its own does not
  > concern it. A journal of another host, one recording no lock, one it could not roll back, or one
  > at its own journal's name refuses it with an error wrapping `ErrProject` that names it; on a file
  > system without writer locks any journal does.
- API.md N11, residuals: prefix the reused-pid and idle-check residuals with "for a journal recording
  no lock,"; add "concurrent edits of nested packages that create or remove one directory may fail or
  keep a journal."
- API.md O5: "A journal whose writer still runs on this host is left alone." becomes "A journal whose
  writer still runs on this host (N11: one of its locks is held, or, recording none, its process
  runs) is left alone; `Open` never waits for a lock."
- API.md S9, appended: "`Open`'s recovery takes them too, without waiting (O5)."
- CLI.md §3.15, the locking sentence becomes: "An edit takes the writer locks of the packages of
  every file it writes, a JSON file's reading packages included (DECISIONS 352, 375), rolls back
  under them an unfinished edit of those packages (API.md N11), and runs API.md N9 under them."
- CLI.md §3.4 and §3.6, appended to each locking paragraph: "Under its locks it first rolls back an
  unfinished edit of a package it locks (API.md N11)."
- CLI.md §3.2, appended: "The new file is created under its package's writer lock, after rolling
  back an unfinished edit of that package (API.md N11)."
- IMPLEMENTATION-PLAN §3: "There are exactly two such seams: `check.Folder` (constant folding during
  checking, §4.7) and `eval.Host` (loading and verification during evaluation, §4.8)." becomes "There
  are exactly three such seams: `check.Folder` (constant folding during checking, §4.7), `eval.Host`
  (loading and verification during evaluation, §4.8) and `build.Settler` (rolling back an unfinished
  edit before a build writes, ADR-0021, installed by `workspace`)."
- DECISIONS:
  > 375. **Writer locks decide an edit journal's liveness and reach (API.md N10, N11, O5, S9; CLI.md
  > §3.2, §3.4, §3.6, §3.15; ADR-0010, ADR-0021; amends 230, 352).** A journal is version 2 and adds
  > `locks`, the sorted packages whose writer locks its writer holds exclusively from before writing
  > it until after removing it; one without (version 1, `Options.FS`, a platform without the
  > primitive, a written file of no package) covers every package and is judged by host, pid and the
  > in-process registry as before. On this host a journal with locks is its writer's exactly while
  > one is held: `Recover` at `Open` takes them without waiting, all or none, and otherwise leaves
  > it, `Open` succeeding. An edit, a build that writes, `canon fmt` and `canon new`, once holding
  > their locks, take exclusively every package of each journal of this host meeting the packages
  > they hold in either mode (freeing theirs and taking the union in name order until stable), roll
  > those back, then go on: an edit whose recovery changed a file is `*StaleError`; a build re-reads
  > (352); fmt refuses a file changed since read. Disjoint journals concern no writer. Another host's
  > journal, one without locks, an undecodable or unknown-version one, or one at the commit's own
  > name still refuses a writing `Edit`; an empty or invalid lock list refuses the journal. An edit
  > locks the readers of each JSON file it writes and the package of each `canon.lock`. Reason:
  > v0.1.8 recovered with no lock, so a
  > rollback could interleave with a live build, fmt or second recovery (lost updates, wedged
  > journals), and any journal blocked every commit (a transient `ErrJournal`, log-2026-10-13).

**Tests.** Edit tests use a fake `Locker` and are deterministic; workspace tests use the OS file
system, real `flock` and a helper process that stalls after its first rename. Each cites its rule.

| Test (package) | Proves | Rule | Today |
|---|---|---|---|
| `TestRecoverLeavesHeldJournal` (edit) | v2 `{a}`, pid dead, `a` held: no change, nil | O5, N11 | rolled back |
| `TestRecoverAllOrNoneLocks` (edit) | `{a,b}` with `b` held: journal left, `a` released | N11 | n/a |
| `TestRecoverRereadsUnderLocks` (edit) | the lock callback removes the journal: skipped | N11 | n/a |
| `TestRecoverBadLockList` (edit) | an invalid list: `ErrJournal`, the locker never called | N11 | n/a |
| `TestRecoverVersion1ByPID` (edit) | a v1 journal follows the pid rule as before | J2 | compatibility |
| `TestCommitIgnoresDisjointJournal` (edit) | `Held [b]` with a v2 `{a}` journal: the commit applies | N11 | `ErrJournal` |
| `TestCommitRefusesForeignLocklessOwnName` (edit) | foreign, lockless and same-name journals refuse | J6 | regression guard |
| `TestSettleRecoversMeeting` (edit) | a meeting journal is rolled back, its changed files returned | N11 | n/a |
| `TestMeetingClosure` (edit) | `{a,b}`, `{b,c}`, held `[a]` gives `[a,b,c]` | J4 | n/a |
| `TestTryAcquire*` (writelock) | busy lock; all or none; a second open in one process is busy; `ErrNoLocks` | J3, J12 | n/a |
| `TestEditOtherPackageWhileCommitStalls` (workspace) | an edit of `b` applies while the helper's journal on `a` is present | N11 | `ErrJournal` |
| `TestOpenLeavesJournalWhileLockHeld` (workspace) | helper killed, test holds `a`: `Recover` leaves it; after release it rolls back | O5 | rolled back while held |
| `TestEditRecoversAbandonedJournal` (workspace) | the first edit is `*StaleError` naming the restored files; the retry applies | N11 | `ErrJournal` |
| `TestRecoverThroughAlias` (workspace, Unix) | `Recover` through a symbolic link leaves the live commit's journal | N11 | rolled back |
| `TestEditLocksJSONReaders` (workspace) | an edit through `a` of a JSON file `b` reads waits on `b` | CLI §3.15 | no wait |
| `TestTwoRecoveriesSerialize` (workspace) | the second recovery leaves the journal; a later edit waits for the first | N11 | both run |
| `TestBuildRecoversAbandonedJournal` (build) | a writing build settles a dead edit's journal, then re-reads | J10 | writes over it |
| `TestFmtRecoversAbandonedJournal`, `TestNewSettles`, `TestExplainKeepsLocks` (cli) | fmt, new and explain follow J10 | J10 | n/a |

The existing crash tests (`TestCommitCrash*`, `TestCommitKilledMidway`, `TestEditJournalAtCommit`,
`TestCommitBusy`, `TestRecoverRunningWriter`) stay valid: their sites have no `Locks`.
`TestEditsAcrossProcesses` runs on the OS file system, so its sites get locks with J-WORKSPACE:
J-WORKSPACE updates it to the new rule only where it encoded the old one (two edits of disjoint
packages now both apply). Concurrency tests also run at `GOMAXPROCS=2` and `4`, under `-race`.

**Units, in this order** (one package each):
1. **J-WRITELOCK** (S, sonnet): `internal/writelock/lock.go` (`TryAcquire`, `Lockable`, `Valid`),
   `errors.go` (`ErrNoLocks`), `doc.go` (drop the `Held.Holds` text); tests `lock_test.go`,
   `process_test.go`.
2. **J-EDIT** (M, opus), beside J-BUILD: `internal/edit/journal.go`, `recover.go` (Locker,
   per-journal rules, `Meeting`, `Settle`), `commit.go` (Site fields, `blocked`, own-name check,
   `locks`), `constants.go`, `apply.go` (`Plan.Unowned`), `doc.go`; tests `recover_test.go`,
   `recover_hostile_test.go`, `commit_test.go`, `commit_fs_test.go`.
3. **J-BUILD** (S-M, opus), beside J-EDIT: `internal/build/project.go` (settle field, settler
   type), `reads.go` (`Over` copies it), `writers.go` (`WithSettle`, the loop in `run.lockWriters`),
   `osfs.go` (`OSReading`); test `writers_internal_test.go`. It does not import `edit`.
4. **J-WORKSPACE** (M, opus), after 1-3, STAMP and O2: `internal/workspace/writers.go` (J7's lock set,
   `lockerFor`, the acquire loop), `edit.go` (commit runs `Settle`, returns `*StaleError`, the panic
   path settles its own journal), `process.go` (`Recover` with the Locker; the exported
   `LockAndSettle`), `project.go` (`NewWith` installs `WithSettle`); tests `writers_test.go`,
   `writers_process_test.go`.
5. **J-CLI** (S, sonnet), after 3-4 and O2: `internal/cli/fmt_write.go`, `explain_source.go`, `create.go`
   (`runNew`); test `writers_test.go`.
6. The orchestrator's one-line fixup after J-WORKSPACE: `api/session.go:35` (J11).

**Acceptance.** Every test above passes, and each marked "Today: …" fails on the unchanged code;
`go test -race` on `internal/writelock`, `internal/edit`, `internal/build`, `internal/workspace`,
`internal/cli` and `api` at `GOMAXPROCS` 2, 4 and default; `GOOS=windows` and
`GOOS=darwin go vet ./...`; the release gates of §8 (stress, `fuzz-edit`, the kill tests).

### 2.3 Asset lookups follow `load.dir`'s link bounds (DECISIONS 376)

**Facts.**
- WIRE §6.5 (`spec/WIRE.md:654-661`): links are followed only when their target resolves inside a
  declared root or the project, else skipped with `W7115`; a directory link already on the walk path
  is not followed again; a dangling or looping base is `E7004`; the project root's own ancestors never
  count as links. The code caps a chain at `MaxSymlinkHops = 40` (`internal/project/constants.go:117`,
  then `ErrSymlinkLoop`); the bounds are the project directory and every root, each resolved
  (`internal/load/symlink.go:39-46`).
- Assets: `internal/verify/refine.go:113-163` tries `E3703`, `E3702`, then a lookup (`E3705` under an
  absent root, else `E3701`). `build/assetplaces.go:44` resolves the asset root with `EvalSymlinks`,
  unbounded. `assets.kind` (`build/hosts.go:365-374`) `Stat`s any link wherever it points; a
  dangling or looping link is neither file nor folder, so the value gets `E3701`. This was a
  deliberate 2026-09-28 call ("No bounds check like load.dir's W7115", log-2026-09-28:754-756),
  reversed here.
- Windows: Go lists a junction as `ModeIrregular` (neither `ModeSymlink` nor `ModeDir`); Canon follows
  only `ModeSymlink` (`project/symlink.go:89-101`), so a junction folder is a file name and any asset
  under it is `E3701` (fails closed).

**Problem.** An asset passes when its file is reachable only through a link pointing anywhere on the
machine: its verdict depends on files outside every declared root and the project, unlike `load.dir`.

**Decision.**
1. WIRE §6.5's bound, verbatim: a link met looking an asset up (in the asset root's own path, a
   folder segment, the file itself) is followed only when its resolved target lies inside the
   project or a declared root, the bounds resolved, the project root's ancestors exempt, the final
   target judged.
2. A file reached only through a link outside the bounds is a new **`E3706`** at the value, naming
   the first such link. Rejected: `E3701` (says "not found" about a file the user sees); `W7115`
   (`load`'s warning, and a second finding beside `E3701`).
3. A dangling link, a loop, or a chain past 40 hops counts as absent: `E3701`, as today. Only
   `ErrSymlinkLoop` (a loop, or a chain past 40 hops) makes an asset root unresolved: it then lists
   nothing, so each value is `E3701` (a looping root is no longer an I/O failure). Any other failure
   to resolve keeps the root's lexical name, as today (for example a file system without links,
   `errNoLinks`, `internal/project/fs.go:25-30`).
4. Entries that are not links keep their kind (a directory is a folder, anything else a file).
   Rejected: "regular files only" (Windows cloud placeholders and junctions are `ModeIrregular`).
5. Windows junctions stay as they are (fail closed); treating them as links belongs in
   `project.readLink` for every reader (§9).
6. Cycles need no rule: a lookup walks only the value's own path segments.
7. Manifest `list` lines (WIRE §10) leave out a link the bounds do not follow, or one that dangles or
   loops; the manifest hash therefore changes only for projects that hold such a link.
8. `E3706` is a fit error for the edit API (API.md E14 counts asset errors as fit errors): it joins
   `fitCodes` (`internal/edit/constants.go`) in the Q-EDIT unit, which runs after ASSETS (the audit's
   `diag-code-untested` ratchet counts a code named in production code as reported, so the name must
   land with or after `E3706_1.txtar`).

**Code.**
- `internal/load/symlink.go` exports behaviour-neutral helpers: `type Bounds []string`,
  `LinkBounds(fsys, layout) Bounds` (today's `walkBounds` body), `(Bounds).Within(real string) bool`,
  `FirstLinked(fsys, layout, base project.Path) string` (wrapping `firstLinkedSegment` through a
  throwaway `Loader`, as `Inside` does).
- `internal/build/assetplaces.go` `root`: when the resolved path differs from the lexical one and lies
  outside the bounds, `placedRoot.blocked = FirstLinked(…)`; on `ErrSymlinkLoop` the root is
  unresolved.
- `internal/build/hosts.go`: the bounds computed lazily, once per run; `kind`, for a link, calls
  `project.EvalSymlinks(a.fs, child)` (through `readLog`, so a retargeted link is a change, API S5):
  an error means the entry is not there, a target outside the bounds is recorded in a new
  `dirListing.blocked`, otherwise `Stat(target)`; `Exists` also returns the display path of the
  blocking link.
- `internal/verify/verify.go`: `Exists(root, from, name string) (display string, found bool,
  link string)`; `refine.go`: `lookup` gains a `link` field (entry replay compares lookups with `==`,
  still correct) and `missing` reports `E3706` when `link` is set.
- Test fakes follow the signature: `verify/fixture_test.go`, `entryreplay_internal_test.go`,
  `gen/ts/pipeline_test.go`.

**Spec text.**
- TYPES.md §13.4, the sentence "The root is listed by its real path, links resolved, as `load.dir`
  does (WIRE.md §6.5)." becomes:
  > The root is listed by its real path, links resolved, and every symbolic link on the way to the
  > file (in the root's own path, a folder or the file itself) follows `load.dir`'s bounds (WIRE.md
  > §6.5): it is followed only when its target resolves inside a declared root or the project. A file
  > reached only through a link outside them is `E3706`, naming the first such link; a dangling or
  > looping link holds nothing (`E3701`) (DECISIONS 376).
- TYPES.md refinement table, the asset row: `E3701`–`E3703`, `E3705`, `E3706`. TYPES.md diagnostics
  table, a new row: `| E3706 | error | an asset reached only through a symbolic link outside the
  roots and the project (evaluation, §13.4) |`.
- WIRE.md §10, the `list` bullet: "…the kind known after following links (a link §6.5's bounds do not
  follow, or one that dangles or loops, is not listed)…".
- ERRORS.md: `| E3706 | error | verify | TYPES.md §13.4 | an asset reached only through a symbolic
  link outside the roots and the project (DECISIONS 376) |` and the message row `| E3706 | - |
  path:Text, link:Path | `asset {path} is reached through symbolic link {link}, which points outside
  the roots and the project` |`.
- DECISIONS:
  > 376. **Asset lookups follow `load.dir`'s link bounds (TYPES.md §13.4; WIRE.md §6.5, §10;
  > ERRORS.md E3706).** A symbolic link met looking an asset up (the asset root's own path, a folder,
  > the file) is followed only when its target resolves inside a declared root or the project, bounds
  > resolved as `load.dir`'s, the project root's ancestors not counting. A file reached only through
  > a link outside them is `E3706`, naming the first such link; a dangling link, a loop or a chain
  > past 40 hops holds nothing: `E3701`, as before. Manifest `list` lines leave such links out.
  > Entries that are not links keep their kind (a Windows junction, listed irregular by Go, stays a
  > file name: `E3701` under it). Reason: an asset passed on a file outside every declared root and
  > the project, and its verdict depended on files the build does not declare (log-2026-10-12, F17);
  > log-2026-09-28's "no bounds check" is reversed.

**Tests** (cite TYPES §13.4 and DECISIONS 376; skipped on Windows like `TestAssetExistsSymlinks`):

| Test | Proves | Today |
|---|---|---|
| `TestAssetLinkOutsideRoots` (build) | a file link, a folder link and a root reached through an outside link are each not found and name the link | found |
| `TestAssetListingOmitsOutsideLink` (build) | the manifest `list` hash leaves an outside link out | listed |
| `TestAssetE3706Check` (build_test, real disk) | `canon check` reports `E3706` end to end | no finding |
| `internal/verify/testdata/findings/E3706_1.txtar` | the `E3706` message | no code |
| `TestAssetLinkIntoAnotherRoot` (build) | a link into another declared root is found | pins (passes) |
| `TestAssetLinkDanglingLooping` (build) | dangling and looping links give `E3701` | pins (passes) |

**Unit ASSETS** (S, sonnet, opus review). It owns `internal/build` and, because `verify.Exists`
changes signature in one commit, may also edit exactly: `internal/load/symlink.go` (the exported
helpers), `internal/verify/verify.go`, `refine.go`, its test fakes (`fixture_test.go`,
`entryreplay_internal_test.go`), `internal/verify/testdata/findings/E3706_1.txtar` and its case in
`internal/verify/findings_test.go`, `internal/gen/ts/pipeline_test.go` (the fake only), and one line
in `internal/testkit/progen/testdata/gaps.txt` (§7.2, S3). This cross-package scope is granted
here; nothing else outside `build`.

### 2.4 Output paths keep their declared spelling (DECISIONS 377; extends 357)

**Facts.**
- On macOS or Windows, `out: "Gen"` → `out: "gen"`: `ReadFile(.../gen/s.sql)` finds the old file;
  `spelling` (`internal/build/respell.go:13-34`) returns at line 22 because the case-insensitive
  listing of `gen` holds the exact base name (357 only looks at the file's own name);
  `ownership.owned` (`textemit.go:222-234`) looks the display path up exactly in `canon.outputs`
  (`@out/gen/s.sql` misses `@out/Gen/s.sql`); `judge` reports `E8001` (`place.go:75`). With unchanged
  bytes the file is marked unchanged and `Gen` stays on disk while `canon.outputs` says `gen`, for
  every target.
- 357's mechanism: `spelling` sets `o.respell`; `writer.respell` (`writer.go:80-98`) sets the file
  aside under the run's staging name and renames it to the declared spelling; `unspell` rolls it
  back. Detection is per path (the name resolves, the directory listing does not show it exactly);
  `writtenHere`/`bothListed` (`textstale.go:144-178`) already fold whole paths. Production code never
  probes the disk for case sensitivity. Test technique: `caseFS` (`respell_test.go:17-56`) and
  `foldFS` (`textguards_test.go:249`); real-disk tests skip through `caseSensitive(t, dir)`
  (`textguards_test.go:144`). CI's macOS and Windows runners are case-insensitive.
- Two emits into `Gen` and `gen` are no error today (`E8152` folds whole file paths only,
  `collide.go:27`; `E8008` compares Go `Dir`s exactly, `ir/rules_emit.go:263`): one directory on
  Windows and macOS, two on Linux.

**Problem.** Ownership and spelling are judged on the file's own name only: a case change in any
directory component raises `E8001` (text outputs whose bytes changed) or silently leaves the old
spelling on disk (every target); and two declared spellings of one directory give different trees on
different platforms.

**Decision.**
1. 357 extends to every component of every output path that `canon` may create (the directories
   below the project directory, or below a present root's directory outside it: CODEGEN §2.4's
   creation authority, `dirBounds.allows`, `absent.go:89-101`), and to the file's own name, in every
   target. A directory is renamed by one rename (no set-aside, so a killed build never hides it),
   before any output below it is written, top-down, de-duplicated, inside the all-or-nothing write,
   rolled back in reverse order. Rejected: refusing and asking the user to rename (a second behaviour
   for one meaning); ownership only (the declared spelling never reaches the disk).
2. A file is renamed only when the build may already overwrite it: a text file through a
   `canon.outputs` line naming it under the old spelling (compared by resolved path, as the stale
   judge does), any other target through its marker in the old bytes; otherwise `E8001`. This also
   closes the same-bytes rename of a JSON value (`let Potions` → `potions`).
3. Detection stays per directory: a component is "the same directory under another spelling" when its
   parent's listing has no exact entry, has an entry equal to it ignoring case (`strings.EqualFold`),
   and the declared path resolves. No fold match in the listing (e.g. HFS+ NFD names): left as it is.
   The declared path does not resolve: it is another directory; the declared one is created and 336
   removes the old listed files by hash. Rejected: a global disk probe (sensitivity varies by volume
   and by directory).
4. Two directories that the selected packages' outputs pass through, or one of them and a package
   directory, equal ignoring case, are **`E8152` `dirCase`** on every platform, in `check` too (355).
   It compares the declared `ir.Emit.Dir` prefixes and the package directories (package directories
   count whatever the selection), so the verdict is the same on every machine. It also closes the
   `E8008` gap. It is reported like `E8152` `file`: at the `out` value of the later emit (package
   order, then declaration order) whose directory differs, naming both directories; against a
   package directory, at the emit's `out` value. Rejected: comparing every project emit (it would
   break 355 and 328).
5. Every output below a renamed directory, and every renamed file, is reported `written` (`stale`
   under `--check`). `--only-root R` renames only below R's directory; a rename needed elsewhere is
   `E8028` `change`, naming that directory.
6. A directory holding a package's sources, or above one, is never renamed: point 4 refuses its other
   spelling first, and the planner raises an internal error as a backstop.
7. CLI.md §3.4 gains the line: "Commit a case-only rename with `git mv`: git with `core.ignorecase`
   does not record one made on such a disk."

**Code** (all `internal/build`): `collide.go` (a `dirCase()` check run from `collide` regardless of
`free`; the two `E8152.At` call sites become `E8152.AtFile`, done in S2); `respell.go` (a planner
`r.dirSpellings(outputs)` run in `place` after `throughLinks`/`owners`; `spelling` extended to every
target and whole paths); `write.go` (`change` gains a `dir` kind); `listorder.go` (directory renames
after `first`, before the other changes); `writer.go` (`stage` skips directory renames; `replace` and
`restoreOne` do a single rename; `removeStages` skips them); `place.go` `judge` (outputs below a
renamed directory are `written`); `consumer.go` (`E8028` for a rename `--only-root` may not make).

**Spec text.**
- CODEGEN.md §2.4, a new paragraph after "No output through a link":
  > **Declared spelling** (DECISIONS 377, extends 357). Every directory an output path passes through
  > that `canon` may create (above), and the file itself, are written as `out` and the file name
  > declare. Where the disk holds one under another spelling, equal but for letter case, that is the
  > same one (its directory lists that spelling and not the declared one, and the declared path
  > resolves), the build gives it the declared spelling, a directory by one rename before any output
  > below it is written, inside the all-or-nothing write; each output below a renamed directory, and
  > a renamed file, is reported `written` even when its bytes are unchanged (`stale` under
  > `--check`). A file is renamed only when the build may overwrite it (its marker, or §2.9's
  > `canon.outputs`), else `E8001`. A spelling the listing does not show is left as the disk has it;
  > where the two spellings are two files or directories nothing is renamed. `build --only-root
  > <root>` renames only below its root's directory (`E8028` otherwise). A directory holding a
  > package's sources is never renamed: WIRE.md's `E8152` refuses its other spelling first.
- CODEGEN.md §2.9, the case-only sentence becomes:
  > A case-only rename removes the old name only where it is another file than the one written (a
  > case-sensitive file system); where it is the same file, under another spelling of its name or of a
  > directory on its path (§2.4), the old line grants ownership of the new spelling and the file is
  > given the declared spelling, even when its bytes are unchanged (DECISIONS 357, 377).
- WIRE.md §8.1, added to the `E8152` bullet:
  > Two directories that outputs of the selected packages pass through (any target, any copy, below
  > the directory CODEGEN.md §2.4 lets `canon` create in), or such a directory and a package's
  > directory, whose paths differ only in letter case are `E8152` too, on every platform: one
  > directory on Windows and macOS, two elsewhere (DECISIONS 377).
- CLI.md §3.4: the `git mv` line of point 7.
- ERRORS.md, `E8028`'s meaning becomes "`build --only-root` when the build would change
  `canon.lock` or a `canon.outputs`, write an output outside the named root's directory, or rename a
  directory outside it (DECISIONS 343, 377)".
- ERRORS.md: `E8152`'s meaning becomes "two outputs of a build with the same path, or paths differing
  only in letter case, or output directories, or one and a package directory, differing only in letter
  case"; its message row `| E8152 | - | …` becomes `| E8152 | file | a:Path, b:Path | `outputs
  collide: {a} and {b}` |`; new row `| E8152 | dirCase | a:Path, b:Path | `output directories {a}
  and {b} differ only in letter case` |`.
- DECISIONS:
  > 377. **Output paths keep their declared spelling (CODEGEN.md §2.4, §2.9; WIRE.md §8.1; ERRORS.md
  > E8152; CLI.md §3.4; extends 357).** 357 holds for every component `canon` may create of every
  > output path, directories included, every target: a directory or file the disk holds under another
  > spelling, equal but for case and the same one (its directory lists only that spelling, the
  > declared path resolves), is renamed to the declared spelling (a directory in one rename, before
  > the outputs below it, inside the all-or-nothing write), every output below is `written` (`stale`
  > under `--check`), and a `canon.outputs` line naming the file under the old spelling owns the new.
  > A file is renamed only if the build may overwrite it, else `E8001`; a directory holding sources
  > never. Judged per directory by its listing, never by probing the disk. `--only-root` renames only
  > below its root (`E8028`). Two directories the selected packages' outputs pass through, or one and
  > a package directory, equal but for case are `E8152` `dirCase` on every platform, found by `check`
  > too (355). Reason: renaming `out: "Gen"` to `"gen"` was `E8001` on macOS and Windows and left
  > `Gen` on disk for every target; `Gen` and `gen` emits wrote one directory on Windows, two on
  > Linux (log-2026-10-13).

**Tests** (cite DECISIONS 377 and CODEGEN §2.4; `caseFS.Rename` is extended to move every key under a
directory):

| Test | Proves | Today |
|---|---|---|
| `TestCaseOnlyDirRename` | directory only (bytes changed and unchanged), directory plus file name, two nested directories: no `E8001`, `--check` stale, declared spelling on disk, new list, fresh afterwards | `E8001` or the old spelling stays |
| `TestCaseOnlySpellingEveryTarget` | a JSON `out` directory, and a JSON value renamed by case with the same bytes, get their declared spelling | old spelling stays |
| `TestCaseOnlyDirRenameRollback` (failingCaseFS) | a failure after the directory rename restores `Gen` with the old bytes | `E8001` |
| `TestDirCaseCollision` (mapFS) | two text emits, and two Go emits, into `@out/Gen` and `@out/gen` give `E8152` in `build` and `check` | no finding |
| `TestDirCasePackageDir` | an emit `out: "A/gen"` beside package `a` gives `E8152` and renames nothing | no finding |
| `TestCaseOnlyDirRenameOnlyRoot` | a rename needed above the root gives `E8028`; below it the rename happens | no rename |
| `TestCaseOnlyDirRenameOnDisk` (real disk; skips on Linux, runs on macOS and Windows CI) | the real rename works | `E8001` there |
| `TestCaseOnlyDirRenameSensitive` | on a case-sensitive FS the new directory is written and the old listed files removed by hash | pins (passes) |
| `internal/build/testdata/findings/E8152_<n>.txtar` (next free n) | the `dirCase` message | no variant |

**Unit DIRCASE** (M, opus): `internal/build` only. Risks: a directory rename on Windows fails while
another process holds a file open in it (the build fails cleanly with the I/O error); Unicode folding
beyond `strings.EqualFold` is out of scope.

### 2.5 `NoHook` is filled for the import closure: a documented limit (no DECISIONS item)

**Facts.** `internal/ir/nohook.go:6-21` fills `NoHook` for the packages a selected package imports,
directly or not (`importedClosure`, :24-42), at `stage.go:51`, after `assemble`/`precompute` and
before `finish`/`validate`/`crossPackage`. The only direct reader is `ownerHooks`
(`text_decoded.go:66-72`; it returns before reading for TypeScript), reached through `TextDecoded`
and `ForeignUses` (`foreign.go:111-117`, used by `goplan.go:96`, `cppplan.go:47`,
`foreign_rows.go:82`, `rules_gen_foreign.go:28`, `gen/ts/decode.go:78`). The owner it reads is
`ownerEmit(p, pkg, t)` over `p.Imports` (`foreign.go:309-319`), and `p.Imports` only ever holds
packages of the source-import closure (a type, value or ref target of another package is reachable
only through imports; layer and entry files cannot import, GRAMMAR.md:554). Every generation path
(the write, `E8152` at check time per 355, `E8026` per 341, `unrun`, `--only-root`, an API subset
build, the LSP, and the 356 stray analysis with its own closure) runs on the selected packages' IR.
The only emits left unfilled belong to selected packages no selected package imports, and nothing
reads them.

**Decision.** No code change; the limit is correct by construction and is pinned by a test.
Rejected: filling `NoHook` for every unit (one more name plan per data emit on every check and edit
re-check, with no reader).
- The `NoHook` field in `internal/ir/ir.go:183` (a frozen contract: a comment-only change, as 362
  did for `Open`) gets the comment `// go, cpp data: records and cases (V.Case) whose make hook this
  emit does not write (DECISIONS 363); filled only for packages a selected package imports, the only
  ones an importer reads`; IMPLEMENTATION-PLAN §4.5's sketch gets the same comment.
- `meta/state.md` moves the item from "Owed, small" to the documented limits.

**Test.** `TestNoHookSelection` (`internal/ir`, package `ir_test`, on the `textdecoders_targets`
fixture; cites DECISIONS 363 and 328): for the selections all, `{a}` and `{a, d}`, package `a`'s
decoders are identical in every target, and every go/cpp data emit reachable through each built
package's `Imports` has the same `NoHook` (`[Link Step.hop]` for `d`). It passes today and fails if
the fill is ever narrowed to the selected packages or moved after `finish`.

**Unit NOHOOK** (XS, sonnet): `internal/ir` (the test and the comment); the IMPLEMENTATION-PLAN
comment is the orchestrator's (S1).

### 2.6 Quoted words: a data name needs no name in scope (DECISIONS 378; amends 366)

**Facts.**
- LEX-08 (GRAMMAR §4.3): any reserved word names a member, case or key in a WORD position (member,
  case and table-key declarations, the name before `:`, after `.`, patterns, `is`, amend segments,
  annotations, translation keys, view items). In value position only the 28 nameable words are read
  bare. Of the other 22, `none true false self` never name data (`E1126`). The remaining 18, **`and
  as break continue else fn for if in is let load match not or return where while`**, can name a
  member, case or key but have no bare value spelling; LEX-08 says such a member "is written
  qualified (`Icon.return`)". It is used: `examples/telemetry/telemetry.canon:189` declares
  `enum Leg { out, in }` and `events.canon:59` writes `leg: Leg.in`.
- There is no escape form: GRAMMAR §1 makes `` ` `` (with `#`, `$`, `&`, `;`, `^`, `~`, `\`, `'`)
  `E1106` outside a literal.
- A layer file cannot import: `layerFile = packageClause NL "layer" IDENT … { amendDecl … }` (§5.2);
  an `import` there is `E1127` (`internal/syntax/parse_file.go:189-199`). Amend values evaluate in the
  package's scope (EVALUATION §9.3) with the layer file's env (`internal/check/layers.go:27`), which
  sees package names, the file's imports (none) and built-ins (`check/scope.go:143`). An entry file
  may import, but an edit never adds an import (366) and a new entry file holds only the package line
  and the entry (API N5).
- The edit printer (`internal/edit/qualify.go:67-88`) writes a non-nameable name `typeName(T).name`:
  `T` in the type's own package, else the name the target file's import binds, else
  `lastSegment(pkg).T` ("for the re-check's E2102"). Layer lines and new entry files use
  `bareNaming()` (`qualify.go:34`; called from `applylayer.go:32, 176`, `place.go:256`). The checker
  meets `v.Word.if`, finds no qualifier `v` (`check/chain.go:68-76`) and reports `E2102` `unknown name
  v` (`scope.go:196-208`).
- Committed tests encode the failure: `TestReservedMemberLayerForeign`
  (`internal/edit/qualify_test.go:253-280`, `[E2102]`); `TestReservedMemberRecheck` (:61-62, goldens
  `reserved_member_unbound` and `reserved_member_entry_file` expect `[E2102]`).

**Problem.**
- An edit fails `E2102` `unknown name <last package segment>` when the target file binds no name for
  the type's package, the type is another package's, the edit writes a member, a case (alone or a
  typed literal such as `v.Shape.while { r: 2 }`) or a dependent symbol pinned to a foreign branch
  enum, and its name is one of the 18 words. It hits layer files (not even a human can write it
  there), entry files an edit creates, and any file whose imports do not bind the package (only
  `import v { Row }`). Ref keys, map keys, table keys and the file's own package's types are not
  affected.
- Two more layer-edit bugs in the same code, found by reading (no test yet): an amendment path copies
  the API path text (`amendPath`, `internal/edit/applylayerline.go:244`), so a key into an enum-keyed
  map or keyed list is written `byWord[if]: 3` (`E1116`); and a `{String: V}` key `a` is written
  `weights[a]`, which the checker types as a name (`check/layers.go:216-217`, `E2102`). Keys holding
  `{` are JSON-escaped, not Canon-escaped.

**Decision.**
1. **Quoted words**, written between backticks: `` leg: `in` ``. Every data name gets a bare value
   spelling, resolved by step 1 exactly as `check` resolves a bare member; no import, type or package
   name is needed, so the text is the same in every file and import tiers (§4) never interfere. Only
   text that is `E1106` today gains a meaning.
2. **The backtick**: free, and the Kotlin, Swift and Scala convention. Rejected: `'w'` (reads like a
   string beside `"w"` union literals), `#w` (`#` is the `[#n]` token), `.w` (shorthand lambda), `@w`,
   `$w`, `\w`, `r#w`.
3. **A new token class `QWORD`** = "`" WORD "`", one token (no space, comment or line break inside),
   valid only as a `primary`: values, list elements, arguments, `[ ]` index and amend keys,
   keyed-item keys, interpolations, typed-literal names (`` `while` { r: 2 } ``). In a WORD position a
   QWORD is `E1116` (the bare word already works there). It is no keyword: it can end an item and
   never continues a line, so DECISIONS 211's comma is never needed after it.
4. **Only the 18 words may be quoted.** A well-formed quote of anything else (an `IDENT`, a nameable
   word, `none`, `true`, `false`, `self`, `_`) is a new **`E1138`** (package `syntax`, one variant); a
   malformed backtick stays `E1106`. A QWORD carrying `E1138` parses as a Bad expression: nothing
   else is reported at it (no `E2102`, DECISIONS 141). Rejected: `E1125` (another meaning); `E1116`
   (it would not state the way out, 305).
5. **Meaning**: an identifier of that name (`IdentExpr{Name: inner}`); no checker code changes
   (`ident()` reads `IdentExpr.Name`, `check/ident.go:10`).
6. **`` `in`: v `` inside braces** is a keyed item (maps already accept `in: v` and `Leg.in: v`).
7. **Printer rule (API M7).** Members, cases (alone or before their fields), `Kind` values, dependent
   symbols and ref word keys are printed bare or quoted, never qualified, the same text in every
   file. Map keys stay named items. Ref keys move from `"if"` (366) to `` `if` `` (one rule for every
   word in value position). A `Source` still accepts `T.n`, `p.T.n` and `"if"` for an enum-keyed ref,
   so stored Undo ops keep working.
8. **Key rename (API §8.4 E11).** Identifier references are rewritten bare or quoted (today they
   become strings); an expression map key `Q.m` keeps its written qualifier and only `m` changes; a
   parenthesised key gets the new word inside the parentheses; any other expression key becomes
   `(word)` or `` `w` ``; strings stay strings.
9. **Source text (API E26).** A `Member` or `Case` lit named by a quotable word marshals as `` `if` ``;
   `ErrNoText` stays for `none`, `true`, `false`, `self`, and for non-finite floats, sub-millisecond
   durations and a FromJSON nested in another value.
10. **Dependent symbols.** A quotable symbol with no computable branch is kept symbolic and printed
    quoted, as a nameable one is today (amends 366); a qualifier naming another branch's type stays
    `ErrBadValue`.
11. **Amendment paths (API §6.4).** Each key is printed as a Canon key of its key type: an integer in
    decimal; a `String` key (a ref whose target is String-keyed, a union literal) as a string literal
    printed as M7 prints strings; another word key bare or quoted; anything else as a string literal.
    This also fixes `byWord[if]` and `weights[a]`.
12. **Existing qualified text stays valid and untouched**: no formatter rewrite, no warning; the
    examples do not change.
13. **Still refused**: `E1106`, `E1138`, `E1116` (a QWORD in a WORD position), `E1125`, `E1126`;
    `E2102` for a quoted word nothing gives a meaning (`` let x: Int = `in` ``); 366's Source refusals
    (an import alias, another branch's type, another type).
14. **The language server** needs no code change: it serves hover, definition, references and
    formatting (`internal/lsp/server.go:228-250`), no semantic tokens; definition and hover on a
    quoted word answer as on a bare member.
- **Rejected:** imports in layer files with edits adding imports (a grammar change; new edges against
  `E2002`, API E17's affected set and import tiers; writes outside the edited item, M6; alias choices
  on `E2005`; an Undo removing imports); a full path without an import (`game.vocab.Word.if`: a
  leading segment can be a let, local or field, `a` and `a.b` make prefixes ambiguous, and every
  cross-package name gains a second spelling, 319); a nameless form in today's syntax (none exists: a
  string is refused for an enum, V4, and is the literal in a literal union); a contextual type
  qualifier resolving `Word.if` through the expected type (asymmetric for the 18 words, or it changes
  valid programs, and it collides with a declared or imported `Word`); forbidding the 18 words as
  names (breaks `Leg { out, in }`, against LEX-08 (a): data mirrors wire values); resolving layer
  values in the let's file scope (hidden coupling, against EVALUATION §9.3, nothing for entry files).

**Code.**
- `internal/syntax`: `constants.go` adds `TokQWord` right after `TokIdent`, named `"QWORD"` (below
  `TokEllipsis`, so `expected()` names it as a class, and below `KwAnd`, so `isWord`, `separate.go:143`,
  is false for it); `tables.go` adds a `quotable` tokenSet of the 18 words and `TokQWord` to
  `startsItemTok`; `lex_code.go` sets `codeTable['`'] = (*lexer).quotedWord` (a well-formed quote
  emits `TokQWord`, plus `E1138` when the word is not quotable; otherwise `E1106` and `TokIllegal` for
  the lone backtick, as today) and exports `IsQuotable(TokenKind) bool` and `ValueText(word string)
  (string, bool)` (the word for an IDENT or nameable word, `` `word` `` for a quotable one, false
  otherwise: the one LEX-08 helper the edit printer uses); `parse_primary.go` sets
  `primaryTable[TokQWord] = (*parser).quotedExpr`, returning `&IdentExpr{Name: inner}` (typed literals
  then work unchanged through `qualifiedOf`). The AST is unchanged (no frozen-contract change).
- `internal/edit`: `qualify.go` deletes `naming`, `fileNaming`, `bareNaming`, `typeName`,
  `importedName` and `qualified`; `nameText` prints `Member`, `Symbol` and `CaseKind` through
  `syntax.ValueText`; `caseText` stands alone; `pinned` replaces `valueWord` with a "has bare text"
  check (point 10); `selected`, `qualifiedName`, `branchSymbol`, `namesType` and `lastSegment` stay
  for Source input. `print.go` drops the `in` field, `refText` uses `ValueText`, `CaseKind` goes
  through `nameText`, `key()` is unchanged. `printlit.go`: `Case`, `Member` and `PathKey` use
  `ValueText`. `applyrename.go` `tokenText` rewrites an IdentExpr with `ValueText`;
  `applyrenamekey.go` `exprKeyText` implements point 8 (a selector key replaces only `sel.Name`). The
  naming arguments are removed at `applystaticadd.go:28,48,84`, `applystaticops.go:118,177`,
  `applysite.go:22`, `applylist.go:142,150`, `applylayerline.go:90`, `applylayer.go:32,176`,
  `place.go:256`, `undolayer.go:193`. `applylayerline.go`: `amendPath(canonical)` becomes
  `amendPathText`, walking `x.res.Steps` (`Seg`, `Container`) plus the new entry's segment and
  applying point 11; `amendLine` keeps `canonical` for its descendant check.
- `editors/vscode`: a `quoted-word` rule in `expression-core` before `#keywords`: the backticks get
  `punctuation.definition.quoted-word.canon`, the word `variable.other.enummember.canon`.
- `internal/testkit` (progen): a `quotedWords` pool and a branch in `namePrimary`
  (`grammar/constants.go`, `gen_lit.go`).

**Spec text.**
- GRAMMAR.md §1, the last two sentences of the third bullet (the `E1106` sentences) become:
  > Any other character, and the ASCII characters `` ` `` `#` `$` `&` `;` `^` `~` `\` `'` outside a
  > literal, is `E1106`, with two exceptions: the `#` of a position segment `[#n]` in an amend path
  > (§5.7), where the lexer produces a `#` token (directly after `[`; a `#` token anywhere else is
  > `E1106`), and a backtick that opens a quoted word (§2.3).
- GRAMMAR.md §2.1, a new row: `` | `QWORD` | `` `in` `` `` `return` `` | §2.3; a quoted word, one
  token | ``.
- GRAMMAR.md §2.3, appended:
  > ```
  > QWORD = "`" ( WORD | "_" ) "`"     (one token: no space, comment or line break inside)
  > ```
  > - A **quoted word** is a reserved word between backticks: `` `in` ``. Its name is the word
  >   without the backticks. It is accepted only as a `primary` (§5.11), read as an identifier of
  >   that name (TYPES.md §3.3); it is never a keyword, so it can end an item and never continues a
  >   line (§3.1).
  > - The **quotable** words are the reserved words that are not nameable (§4.3) and may name a
  >   member, case or key: `and as break continue else fn for if in is let load match not or return
  >   where while`. Quoting any other word (an `IDENT`, a nameable word, `none`, `true`, `false`,
  >   `self`, `_`) is `E1138`.
  > - A backtick not followed, in one token, by a letter or `_`, word characters and a closing
  >   backtick is `E1106` at the backtick.
- GRAMMAR.md §2.9, a new line: `` '`' -> read a quoted word (§2.3): E1106 if no word and closing
  backtick follow, E1138 if the word is not quotable ``.
- GRAMMAR.md §4.3, a new table row: `` | (d) value position, the quotable words, quoted (§2.3) |
  `QWORD` | `` leg: `in` `` | ``; and "a member with such a name is written qualified
  (`Icon.return`)." becomes "a member, case or key with such a name is written quoted there
  (`` icon: `return` ``, §2.3), which needs no name in scope, or qualified (`Icon.return`, TYPES.md
  §4.3)."
- GRAMMAR.md §5.11, the primary rule: `| IDENT | nameable | QWORD   (* §4.3 (c), (d) *)`.
- GRAMMAR.md §12: `| E1106 | error | a character no token can start with (§1), a `#` outside an
  amend-path position segment, or a backtick that opens no quoted word (§2.3) |` and `| E1138 |
  error | a quoted word that is not quotable (§2.3) |`.
- TYPES.md §3.3, a paragraph after the numbered list:
  > A quoted word (GRAMMAR.md §2.3) is an identifier whose name is the word between its backticks,
  > looked up as that identifier would be. No declaration, local, parameter, field, import or package
  > is named by a quotable word (`E1125`), so steps 2 to 6 never find one: it is a member, case or
  > static key of the expected type (step 1, §4.1), a symbolic key of a `ref` with dynamic keys, or a
  > symbol of a dependent type (§11.4); anything else is `E2102`. It needs no import: `` leg: `in` ``
  > in a layer file names the member `in` of the expected enum, whatever its package.
- API.md §6.4, the body becomes:
  > When `Options.EditLayer` is set (§7.5), the edit path is translated into an `amend` path
  > (GRAMMAR.md §5.7): a field or a table entry whose key is a word is `.name` (the first bare); every
  > other key is a Canon key of the container's key type (for a `ref`, its target's key type): an
  > integer in decimal; a `String` key, or a literal of a literal union, as a string literal printed
  > as M7 prints strings (`["a"]`, `["a b"]`); any other key that is a word, bare or quoted
  > (`[plain]`, `` [`if`] ``, GRAMMAR.md §2.3); anything else as a string literal. A path that has no
  > `amend` equivalent is `ErrNotEditable` with reason `layer`.
- API.md §8.2, the parenthesis of the Source row becomes: "(no names but contextual ones, SPEC §6.2,
  bare or quoted (GRAMMAR.md §2.3), and an enum member or case qualified by its type, `T.n` or
  `p.T.n` with `p` the package's path or last segment; DECISIONS 366, 378)".
- API.md E26, the last sentence becomes: "A `Member(name)` or `Case(name)` named by a quotable word
  is written quoted (`` `if` ``, GRAMMAR.md §2.3); one named `none`, `true`, `false` or `self`, which
  name no member or case, has no `Source` text: `ErrNoText` (DECISIONS 378)."
- API.md M7, the first sentence becomes (the rest unchanged from "Durations use…"): "**M7.** Values
  are printed with contextual names (SPEC §6.2): enum members, case names (alone or before their
  fields), `Kind` values, dependent symbols and keys bare, one named by a quotable word quoted
  (`` `return` ``, GRAMMAR.md §2.3), the same text in every file whatever its imports; a member or
  case is never printed qualified, and a map key is the WORD before `:` (DECISIONS 378)."
- SPEC.md §2.4: "…bare in value position for the words that cannot start an expression
  (`icon: check`), and quoted there for the others (`` icon: `return` ``), which needs no import."
- ERRORS.md: `E1106`'s meaning becomes "a character no token can start with, a `#` outside an
  amend-path position segment, or a backtick that opens no quoted word"; new rows `| E1138 | error |
  syntax | GRAMMAR.md §2.3 | a quoted word that is not quotable (an identifier, a nameable word,
  `none`, `true`, `false`, `self` or `_`) |` and `| E1138 | - | word:Name | `"{word}" is written
  without backticks` |`.
- FORMATTER.md: no change (a QWORD is a leaf printed as written).
- DECISIONS: 366 gets "[amended by 378]", and:
  > 378. **Quoted words: a data name needs no name in scope (GRAMMAR.md §1, §2.3, §4.3, §5.11;
  > TYPES.md §3.3; API.md §6.4, §8.2, E26, M7; SPEC.md §2.4; ERRORS.md E1106, E1138; amends 366).** A
  > reserved word that names a member, case or key but cannot be read bare in value position is
  > written there between backticks, one token: `` leg: `in` ``. The quotable words are `and as break
  > continue else fn for if in is let load match not or return where while`; a quoted word is an
  > identifier of that name, so only step 1, a symbolic key or a dependent symbol give it a meaning,
  > else `E2102`; it needs no import, type or package name, in a source, entry or layer file alike. It
  > stands only where an expression does; quoting another word is `E1138`; a backtick that opens no
  > quoted word stays `E1106`. Qualified forms stay valid and are kept as written. Edits print every
  > member, case, `Kind` value, dependent symbol and ref word key in value position bare or quoted,
  > the same text in every file, never qualified; a renamed expression map key keeps its written
  > qualifier; an amendment path writes each key as a Canon key of its key type (`["a"]`,
  > `` [`if`] ``). A `Member` or `Case` lit named by a quotable word has `Source` text; a quotable
  > dependent symbol with no computable branch is kept symbolic, as a nameable one. Reason: owed since
  > v0.1.6 (log-2026-10-13): a layer has no imports and an edit adds none, so another package's `if`
  > member could not be written there, by edit or by hand; the printed qualifier depended on each
  > file's imports; amendment paths wrote `byWord[if]` and `weights[a]`.

**Tests** (each cites its rule; "Today" is the result on `6a5fc5f3`):

| Test | Package | Proves | Today |
|---|---|---|---|
| `testdata/findings/E1138_1.txtar` | syntax | quotes of `check`, `plain`, `none`, `true`, `_` are each `E1138` (§2.3) | `E1106` |
| `testdata/findings/E1106_<n>.txtar` | syntax | a lone backtick, `` `in `` unclosed, an empty quote, `` `a b` ``, `` `9x` `` are each `E1106` (§1) | pins the boundary |
| `testdata/parse/quoted.txtar` | syntax | AST of a value, a list, a keyed item, a typed literal, an argument, an index, an interpolation, an item ending in `` `in `` with no comma (§2.3, §3.1, §5.11) | `E1106` |
| `TestQuotableWords` | syntax | for every §4.1 word: one QWORD exactly when quotable; quotable = reserved − nameable − {none, true, false, self} (LEX-08) | no token class |
| `testdata/fmt/quoted.txtar` | format | a fixed point; the quote kept; a broken list item ending in `` `in` `` gets no comma | does not parse |
| `testdata/accept/quoted.txtar` | check | package `v` (`enum Word { plain, if, while, in }`, `variant Shape { dot, while { r: Int } }`); package `p` with a source importing only `v { Row }`, an import-less entry file and a layer file using a value, a list, a typed literal, a dependent symbol, a ref key, a keyed map key, `` byWord[`if`]: 3 `` and an interpolation: no finding (TYPES §3.3) | `E1106` |
| `testdata/findings/E2102_<n>.txtar` | check | `` let x: Int = `in` `` gives `unknown name in` | `E1106` |
| `TestReservedMemberRecheck` | edit | `reserved_member_unbound` and `reserved_member_entry_file` now expect nil; three new goldens below, nil | `[E2102]` |
| `TestReservedMemberLayerForeign` | edit | writes `` loc: `return` `` and `` word: `if` ``, no error (W11, 378) | `E2102` |
| `TestReservedMemberSourceText` | edit | `if`, `return`, `while` marshal quoted; `Member("true")`, `Member("none")` stay `ErrNoText` (E26) | `ErrNoText` |
| `TestReservedSymbolNoBranch` | edit | `{ param: WB.if }` and `Member("if")` give nil (point 10) | `ErrBadValue` |
| `TestQuotedSource` (new) | edit | `` `if` `` and `` [`if`, plain] `` accepted; `` `plain` `` is `ErrBadValue` (§8.2) | `ErrBadValue` |
| golden `quoted_layer_keys.txtar` | edit | a layer Set through an enum-keyed map gives `` byWord[`if`]: 3 ``; through an enum-keyed list `` rows[`while`].n: 2 `` (§6.4) | `E1116` |
| golden `layer_string_key.txtar` | edit | Set `conf.weights[a]` gives `weights["a"]: 5`; AddEntry with key `"x{y"` gives `["x\{y"]` (§6.4) | `E2102` (if it passes today, it stays as a regression test) |
| golden `quoted_case_entry_file.txtar` | edit | an `@files` entry gives `` entry filed.a { shape: `while` { r: 2 }, word: `in` } `` (N5, 378) | `E2102` |
| `testdata/quoted_word.txtar` | lsp | definition and hover on `` `if` `` in a layer file answer as on a bare member | n/a |
| two `lexicalCases` | testkit/vscodegrammar | the quoted word is scoped as an enum member, never as a keyword | n/a |

The 17 non-refusing `reserved_*` edit goldens are regenerated with `-update`; their only diffs may be
member, case, symbol and ref-key texts (and the Undo Sources) becoming quoted; the two
`refuse_reserved_*` goldens do not change. `TestEditGoldens` checks Undo (E22) and M6 on every golden.

**Units.** **Q-SYNTAX** (S-M, sonnet, opus review): `internal/syntax`, plus `E1138`'s line in
`internal/testkit/progen/testdata/gaps.txt` (§7.2, S3). **Q-EDIT** (M-L, opus, after J-EDIT, Q-SYNTAX
and ASSETS): `internal/edit`, including `diag.E3706` in `fitCodes` (§2.3 point 8). After Q-SYNTAX,
XS, sonnet: **Q-FORMAT** (`internal/format`, test only), **Q-CHECK** (`internal/check`, tests only;
after N-EVAL too, §7.2), **Q-LSP** (`internal/lsp`, the test only), **Q-EDITORS** (`editors/vscode`
and its test in `internal/testkit/vscodegrammar`), **Q-PROGEN** (`internal/testkit/progen/grammar`
pools). The guide text is the GUIDE unit's (§7.2). A test-only unit that exposes a bug in its own
package fixes it there (Q-FORMAT in `internal/format`, Q-LSP in `internal/lsp`); a lexer or parser bug
goes back to Q-SYNTAX (resume the same builder).

### 2.7 v0.1.4's cross-root trap and ConstantID clash: already closed

Both shipped in v0.1.6 (reply 2026-10-13, F3); the "owed" lines in `meta/handoff/README.md:40-41`
and the plan were stale. The cross-root import trap is DECISIONS 347 (`E8004` `consumerCopy`;
`internal/ir/copies_test.go` `TestConsumerCopyAllowed`, `TestCopyOfConsumer`; findings
`E8004_6.txtar`, `E8004_7.txtar`). The ConstantID clash is DECISIONS 348 (one string id type per name
in `data` and `types` mode; `baked` keeps `E8005` `idType` with its way out;
`internal/ir/testdata/e2e/lifted/shared_id_type.txtar`). No work in v0.1.9; the docs are corrected
(§7, S1).

## 3. N11: positions are not ids (DECISIONS 379)

Louis's frame (log-2026-10-14, "N11 corrected"): safety by design first, detection second. (1) a
spec rule; (2) generated index types are opaque and serializers write the key; (3) a warning when an
entry position reaches an output, with an explicit opt-in; (4) a read-only Resource/Source scan
first. The scan is done (below); this section is the result.

**Facts: what the generated code exposes** (Canon side).
- Canon generates no serializer in any target: Go, C++ and TypeScript output loaders and decoders only
  (no `MarshalJSON`, `MarshalText`, `to_json` or `JSON.stringify` in `internal/gen/{go,cpp,ts}` or their
  runtimes). Canon's own writers write keys: refs as keys (WIRE §5.9), rows with `$id`, an enum as its
  wire value (its code only under `@json(codes)`, which needs `@codes`).
- **Go, `baked` mode:** a table id is the entry's position: `type StatusID uint8` with constants 0..5 in
  entry order (`examples/teamboard/expected/sovcommon/teamboard/teamboard.gen.go:190-199`); its only
  method is `String()`, plus the function `ParseStatusID`. `Get(id)` is `&self.rows[id]` (:927); lookup
  tables are indexed by id (`canTransitionTable[from][to]`, :1440); other packages index by foreign
  ids (`internal/gen/go/testdata/emberfall/baked/game/world/out/go/world.gen.go:433` `levelOfTable[s]`,
  :128 `self.costFor[s]`) and convert ints to foreign ids (`emberfall/data/.../world.gen.go:1067`
  `core.StatusID(j106)`). So `encoding/json` writes the position (and a `[]StatusID` as base64
  bytes), `database/sql` stores it as an int64, `gob` writes the integer, and `StatusID(3)`,
  `var s StatusID = 3`, `CanTransition(0, 1)` and `id+1` all compile.
- **C++, `baked`:** `enum class StatusId : uint8_t {…}` whose enumerators are positions
  (`examples/features/shared/expected/shared/out/cpp/core/core.gen.h:114-118`); `Get` uses
  `static_cast<size_t>(id)` (:363). nlohmann (3.9 and later) writes and reads any enum as its
  underlying integer, silently.
- **TypeScript:** ids are unions of keys (`JSON.stringify` writes keys), but
  `export const StatusIdIndex` exports a key → position table
  (`examples/teamboard/expected/web/teamboard/generated.ts:91-92`), used by own lookups (:325),
  foreign lookups (`internal/gen/ts/fns.go:186`) and foreign decoders' `decEntry`
  (`decode_value.go:307`, `runtime/decode.ts.txt:187-191`).
- `data` and `types` modes: ids are key strings in every target (348). `At(i)`/`at(i)` positional
  lookups exist by rule (SPEC §15.1). `gen/json` writes rows as an array in entry order with `$id`.
  The view model shows row order and a 1-based `{index}` (VIEWMODEL §3.4); it is no persisted output.
  The lock is sorted by bytes (LOCK §2.3) and the fingerprint excludes entries and keys
  (FINGERPRINT.md:77): no position there.
- **Positions in the language:** only `enumerate()` (STDLIB §4.2) and `indexOf` (§4.1) produce them;
  `at(i)`, `xs[i]` and slices consume them; enum `.index` (§3) is a declaration position. `findIndex`
  is not a Canon function (it is the evaluator's helper behind `first(pred)`/`any(pred)`,
  `internal/eval/std/seq.go:49`). `[#n]` exists only as a path segment (amend paths, EVALUATION
  §9.2; API/CLI paths): it selects an entry by position and never yields a value. Each Int that
  `enumerate` pairs with an element carries the call's `Prov` (`seq.go:242-248`).
- **`value.Prov`** is `{Kind uint8, Span (12 B), Via, Ext}`, 32 bytes with 3 padding bytes after
  `Kind` (`internal/value/prov.go:14-19`); site Provs are interned by (kind, span)
  (`internal/eval/reprov.go:62-81`); memo fingerprints hash Prov fields (`memo_fp.go:237-250`).
- **"File order affects only generated row order" is false.** Entry order (the literal's entries,
  then entry files in path order, SPEC §4.3) also decides what iteration computes (the order of
  lists, maps and text built from a table), which entry `first()`, `last()`, `at(i)` and
  `first(pred)` return, ties in `sortBy`, `min`/`max`/`minBy`/`maxBy` and `topoSort`, `groupBy` and
  `toMap` key order, JSON row, nested-table and `$`-key order, `@text` bytes and `canon.outputs`
  hashes, view-model order, and which location `E3101`/`E3102` name first. It never changes keys,
  fields, refs, the lock or the fingerprint.

**Facts: the consumer scan** (read-only, 2026-10-14; Resource 8,313 Canon files, Source, the services).
- Nothing persists a Canon position today: ids leave as `.Wire()` keys or `@codes` codes, DB columns
  are text, JSON wire structs are string-typed (no field of a generated type has a `json` or `db`
  tag), and no consumer imports a TS `<X>Index` table.
- The hazard is latent: Go positional types are bare `uintN` with no `MarshalText`/`Valuer`, so a
  future `json.Marshal`, SQL argument or `%d` would write the position. Resource has 5 Go `baked`
  ids (`commonmigrate.StreamID`, `feedcat.EntryID`/`StackID`,
  `progressionrecordbook.RecordBoardID`/`RecordGroupID`; more in website, trello and teamboard copies
  not on disk), 108 Go enums without `@codes`, 145 C++ enums (all `enum class`), 355 TS `<E>Index`
  tables.
- Resource uses of positions: `enumerate` 9, `.at()` 9 (tests and checks), enum `.index` 4 (checks)
  and 2 (test pins), `for i in 0..xs.len()` 29, `indexOf`/`[#n]`/`@json(pairs:)` 0, `emit json` 0.
  Only one reaches an output: `telemetry/sql.canon:146-147` writes `i + 1` over a plain list
  (`provenance[i]`) into `read_macros.sql` (a view recreated on every load; nothing reads the
  `ordinal`).
- Order coupling outside Canon's reach: the feed-cursor leg `#index` (monitoring `feedlegs.go:31`,
  guarded by an append-only `feedLegOrdinals` registry), `PointTypeKindMembers[0]` labels in the
  admin web UI (`members/[id]/+page.svelte:276-277`), the telemetry X-macro column order, the VIP rows
  ("the array index IS the level", `VipConfig.cpp:71`). They go into the reply as advice (§8).
- Cost of opaque baked ids for Resource's consumers: Go 0 known sites (the 1 production and 2 test
  sites the scan found involve enums, which stay unchanged, point 1 below); C++ 0
  (`RecordBookV2Config.h:261 static_cast<size_t>(id)` stays legal); TS 0.

**Decision.**
1. **Scope: entry positions, so `baked` (and `embedded`, when built) table ids, in every target.**
   Enums keep their numbers in v0.1.9: their order changes only by an edit of one declaration, which
   review sees; a persisted enum number is already `@codes` (WIRE needs `@codes` for numeric
   enums); making Go enums opaque would rewrite every enum golden and every `switch`; and no consumer
   leak was found. One cheap guard applies to them now (point 16). The rest is v0.2, agreed with
   Louis on 2026-10-14 (§9): Go enums without `@codes` opaque with `Position()`, C++'s nlohmann hooks
   deleted for them after a read-only scan of Source, TypeScript's `<E>Index` no longer exported. The
   spec rule names enum declaration positions anyway (SPEC §4.3's text, below). TS `<E>Index` tables
   of enums stay exported in v0.1.9.
2. **Go id shape:** `type <R>ID struct{ position uintN }` (unexported field; `uintN` the smallest of
   uint8/16/32 holding the count, as today). Members are `var`s, same names, in entry order.
   `String()` returns the key, switching on `self.position` with constant cases (switching on the
   vars would be O(n)); its fallback prints `<R>ID(<position>)` as today. `Parse<R>ID` keeps its text
   and returns `<R>ID{}, false` on failure. `Position() int` (an array index for one build).
   `MarshalText() ([]byte, error)` writes the key; `(*<R>ID) UnmarshalText([]byte) error` reads a key
   and returns `errors.New("<R>ID: no entry " + strconv.Quote(string(text)))` for an unknown one. No
   integer converts to or from the id; `encoding/json`, `xml` and `yaml` write the key; `database/sql`
   and `gob` fail loudly instead of storing a position. Rejected: a named int (the leak); `type <R>ID
   string` (any string converts, O(1) array lookups lost, memory grows); functions as constants
   (unreadable).
3. **Generated code goes through `id.Position()`** for every array index, in own and foreign packages;
   no generated code converts an int to an id; a domain is iterated as an array literal of the id
   vars (replacing `core.StatusID(j)`). Rejected: per-package maps from id to index (slower lookups,
   more generator paths).
4. **C++ keeps `enum class`** and adds, in the package namespace, `template <typename J> void
   to_json(J&, <R>Id) = delete;` and `template <typename J> void from_json(const J&, <R>Id&) =
   delete;` (no nlohmann include), so nlohmann fails to compile instead of writing the position.
   `declareBakedNames` reserves `to_json` and `from_json`. If `TestIDEnumRefusesNlohmann` shows that
   g++ or clang++ still compiles `json j = StatusId::open;`, the C++ unit drops the deleted pair and
   keeps only the T15 doc lines of point 12; it adds no other mechanism; and in that unit's commit the
   orchestrator removes the deleted pair from CODEGEN §5.3's row, drops the T17 row, and changes
   DECISIONS 379's "C++ keeps `enum class` and deletes `to_json`/`from_json`" to "C++ keeps `enum
   class`, documented to be stored through `ToWire`". Rejected:
   `NLOHMANN_JSON_SERIALIZE_ENUM` (maps unknown keys to the first member, and needs nlohmann in
   `baked` mode); a `to_json` writing the key (no exception-free `from_json`).
5. **TypeScript stops exporting `<Id>Index`** and exports `<Id>Members: ReadonlyArray<Id>` (keys in
   entry order, like `<E>Members`). A file that needs positions declares a private const only when it
   uses it: `const <Id>Index = canonIndex(<Id>Members)` for its own table,
   `const <alias>_<Id>Index = canonIndex(<alias>.<Id>Members)` for a foreign one (lookups,
   `decEntry`). The helper `canonIndex` joins CODEGEN §8.2 after `canonList`, and
   `runtime/helpers.ts.txt` holds exactly the §8.2 text. Import cycles are `E2002`, so module
   evaluation order is safe. Rejected: literal key lists per module (duplicates large tables); nested
   `Record` lookup tables (churn).
6. **Serializers:** Canon's own writers already write keys; nothing changes there.
7. **`At(i)`, `Len()` and `at(i)` stay** (SPEC §15.1); their doc comments do not change.
8. **An entry position** is the Int that `enumerate()` pairs with an element, or that `indexOf`
   returns, when that element is an entry: a record with `Ident != nil` (a table entry or keyed-list
   element) or a `*value.Ref`. Not: a position in a plain list, an enum `.index`, a `Range` counter
   (even `0..t.len()`), `len()`, a position in a list derived from a table by `map`. Detection is the
   second line (Louis): these blind spots are accepted, and the spec names them. Rejected: every
   `enumerate` (false positives on slots and steps); the receiver's static type only (misses
   `items.filter(…).enumerate()`).
9. **Outputs:** everything stage E precomputes for a package with an emit other than `view`
   (`internal/ir/precompute.go:10-22`): emitted values (`unit.values`), stored export fn results
   (`ExportFn.Value`, lookup cells, method `Instances`, which feed `$` keys and `$fns`), and `@text`
   results. Not outputs: the view model, findings and messages, conformance vectors, the lock.
10. **Tracing: a `Position bool` mark in `value.Prov`,** declared right after `Kind` (in the
    padding: `Prov` stays 32 bytes). Sources: point 8. Carried by `+ - * / %`, unary `-`, string
    `+`, a template with a marked part (format specs included), `Int(x)`, `Float(x)`, `String(x)`,
    `join` with a marked element, and `reprov` (spreads and layer amends). A value passed along keeps
    its mark; nothing else carries it (Bools, `at(i)`, `len`). No "positions minted" fast path (a
    memo replay could bypass it). A marked Prov is a twin: interned per (kind, span, position) when
    `Via` and `Ext` are nil, else a 32-byte copy; a Prov is never written after it is attached (373).
    Cost: 0 bytes per value; no allocation unless an entry position is produced. Equality, hashing,
    the text form, `explain`, the API and outputs ignore the mark. Rejected: the Prov span only
    (misses `i + 1`); a static check-time flow analysis (cannot know entries, misses flows through
    functions); a side map from value to mark (a lookup per operand); a new `Position` type (breaks
    every arithmetic use of `enumerate`).
11. **The opt-in: `positional(x: Int) -> Int`** returns `x` unmarked, step cost 0: one form, at the
    point of use, for fields, keys and text. Rejected: a field annotation (nothing to attach to in a
    `@text` string or a map key: a second form); `rowNumber` (suggests +1); `ordinal` (ambiguous); a
    project knob (319).
12. **`W8031`** (package `ir`, stage E), run right after `s.precompute()` (`internal/ir/stage.go:48`):
    a marked value written to an output. Span: the marked leaf's site (`verify.SiteOf`), falling back
    to the output's declaration as `E8102` does (`rules_textforms.go:140-147`); related note: the
    output declaration (`source`). Reported once per (site, package). Its `{output}` argument is the
    qualified name (`pkg.name`) of the emitted `let`, the `export fn` or the `@text` fn. Never an error in v0.1.9; only
    a later DECISIONS item may escalate it. The generated doc comments (CODEGEN §2.5, fixed comment
    T15) say it in the code: Go type "`// <Name> is the key of an entry of <value>: compare it, switch
    on it, look it up with Get.` / `// To store or send it, write its key (String, or MarshalText
    through encoding/json):` / `// its position is not an id, and changes when entry files move.`";
    Go `Position` "`// Position is the entry's place in <value> in this build, for array indexes in
    memory.` / `// It is not an id: it changes when entry files move.`"; C++ enum "`/// The key of an
    entry of <value>. Store or send ToWire(id): the underlying value is` / `/// a position, not an id,
    and changes when entry files move.`"; C++ deleted pair "`/// nlohmann/json would write the
    position: write ToWire(id), read <Id>FromWire(s).`".
13. **`W1912`** (package `check`): an amend `[#n]` whose container is a table, keyed list or map
    warns (select the entry by key); a plain list stays silent. A warning in v0.1.9 because existing
    fixtures use it.
14. **The memo** hashes the mark (`memo_fp.go`), so cold and incremental analyses give the same
    `W8031` (ADR-0011's gate).
15. **Frozen contract.** `value.Prov` belongs to IMPLEMENTATION-PLAN §4.3: the field is added under
    §4's review rule (an opus builder, spec-reviewer as the consumers' proxy, this ADR). Louis's N11
    call (log-2026-10-14, "traced through Prov") is the approval, as 371 and 373 recorded his. §4.3's
    sketch gets the field.
16. **Go enums without `@codes` write their wire value through text encoders** (Louis, 2026-10-14).
    Every closed Go enum without `@codes`, in every mode, gains (shown for `Tone`):
    ```go
    func (self Tone) MarshalText() ([]byte, error)
    func (self *Tone) UnmarshalText(text []byte) error
    ```
    `MarshalText` returns `[]byte(self.Wire())` for a member, and `errors.New("Tone: no member " +
    strconv.Itoa(int(self)))` for any other value. `UnmarshalText` sets `*self` from
    `ParseTone(string(text))`, and returns `errors.New("Tone: no member " + strconv.Quote(string(text)))`
    when that fails. So `encoding/json` (values and map keys), `encoding/xml` and YAML write wire names
    instead of declaration positions. Nothing else changes: the type stays `uintN`, members stay
    constants, explicit conversions still compile (opacity is v0.2), and `database/sql` and `gob`
    still see the integer. Unchanged in v0.1.9: enums with `@codes` (their number is the stable code),
    open enums (DECISIONS 339's representation; v0.2 decides them with the rest), C++ and TypeScript
    enums. The name plan reserves `MarshalText` and `UnmarshalText` on every enum and id through the
    shared `goIDEnumMethods` list (`internal/ir/tables.go:197-199`); `Position` stays id-only.
    Rejected: a `driver.Valuer` (a new import in every generated package, for a path v0.2's opacity
    closes properly).

**Spec text.**
- SPEC.md §4.3, a new bullet after "Entry order":
  > - **Positions are not ids** (DECISIONS 379). Moving, renaming or adding an entry file changes
  >   entry order, so an entry's position never stands for the entry: its key does. Generated id
  >   types are opaque and never convert to or from a position (CODEGEN.md §5.3), and every
  >   serializer Canon generates writes the key. A number that must keep naming one entry across
  >   builds is declared and locked: a `@stable` field of a `stable table`, or an enum's `@codes`
  >   code (§12, LOCK.md §1); `.index` and the number Go and C++ give a member without `@codes` are
  >   declaration positions (Go's text encoders write such a member's wire value, through
  >   `MarshalText`). An entry position written to an output warns (`W8031`, EVALUATION.md
  >   §13.1) unless written with `positional(…)`. Entry order still orders rows and what iterating
  >   the table computes (`first()`, `at(i)`, ties of `sortBy`, `minBy`, `maxBy`, the order of
  >   lists, maps and text built from it); it never changes an entry's key, fields or refs.
- EVALUATION.md, a new subsection at the end of §13 "Provenance":
  > ### 13.1 Entry positions
  >
  > An **entry position** is the `Int` that `enumerate()` pairs with an element, or that `indexOf`
  > returns, when that element is an entry: a table entry, a keyed-list element or a `ref`
  > (STDLIB.md §4.1, §4.2). Its provenance carries a **position mark** (DECISIONS 379). A position in
  > a plain list, an enum's `.index`, a `Range` counter (`0..t.len()` included), `len()`, and a
  > position in a list derived from a table by `map` carry none.
  >
  > The mark is carried by `+`, `-`, `*`, `/`, `%`, unary `-`, string `+`, a template with a marked
  > part (format specs included), `Int(x)`, `Float(x)`, `String(x)`, `join` over a marked element,
  > spreads and layer amends; a value passed along keeps it. Nothing else carries it: a `Bool`,
  > `at(i)`, `len`. `positional(x)` (STDLIB.md §2.4) returns `x` without it.
  >
  > A marked value written to an output (CODEGEN.md §2.10) warns `W8031`. The mark belongs to the
  > provenance, not the value: equality, hashing, the text form, `explain`, the API and outputs ignore
  > it. It is the second line, after opaque generated ids (SPEC §4.3); its blind spots are the
  > unmarked positions above.
- EVALUATION.md's diagnostics table gains `| W1912 | warning | an amend path that selects a table,
  keyed-list or map entry by position (§9.2) |`, written in that table's columns.
- EVALUATION.md §9.2, after the segment table: "`[#n]` on a table, keyed list or map warns `W1912`:
  name the entry by key."
- STDLIB.md, a new subsection after §2.3:
  > ### 2.4 Positions
  >
  > | Signature | Result | Errors | Cost |
  > |---|---|---|---|
  > | `positional(x: Int) -> Int` | `x`, no longer an entry position (EVALUATION.md §13.1): a position written to an output on purpose, such as a legacy row number | | 0 |
- CODEGEN.md, a new subsection after §2.9:
  > ### 2.10 Entry positions in outputs
  >
  > Stage E walks every value it precomputes for a package with an emit other than `view`: the
  > emitted values, the stored results of `export fn`s (lookup cells, method instances, and the `$`
  > keys and `$fns` they feed) and `@text` results, each record once, in traversal order. A value
  > carrying an entry position's mark (EVALUATION.md §13.1) is `W8031`, at the marked value's site,
  > or at the output's declaration when the provenance names no site, with the output's declaration
  > as a related `source` note; once per site and package. `{output}` is the qualified name
  > (`pkg.name`) of the emitted `let`, the `export fn` or the `@text` fn. The view model, findings and
  > messages, conformance vectors and the lock are not outputs here. `W8031` stays a warning; only a
  > later decision may make it an error (DECISIONS 379).
- CODEGEN.md §5.3, the `baked`, `embedded` row becomes:
  > | `baked`, `embedded` | `type StatusID struct{ position uint8 }` (opaque: no integer converts to or from it) + `var`s `StatusIDOpen…` in entry order, `String()` and `MarshalText()` (the key), `(*StatusID).UnmarshalText` (a key; an unknown one is an error), `ParseStatusID(key) (StatusID, bool)`, `Position() int` (an array index for one build, §2.10) | `enum class StatusId : uint8_t { open, taken, … }`, inline `ToWire(StatusId)`, `StatusIdFromWire(std::string_view)` (binary search, in `.gen.cpp`), and deleted `to_json`/`from_json` templates for it (DECISIONS 379) | `export type StatusId = "open" \| "taken" \| …` and `StatusIdMembers: ReadonlyArray<StatusId>` in entry order; no exported index: a file that needs one builds it privately with `canonIndex` (§8.2) |

  and a sentence after the table: "A `baked` id is opaque so that its position, which changes when
  entry files move, never leaves as an id: `encoding/json`, `xml` and `yaml` write the key;
  `database/sql` and `gob` refuse it (SPEC §4.3, DECISIONS 379)."
- CODEGEN.md §2.5's fixed-comment table gains, after T14:
  > | T15 | Go | `baked` id type | `// <Name> is the key of an entry of <value>: compare it, switch on it, look it up with Get.` / `// To store or send it, write its key (String, or MarshalText through encoding/json):` / `// its position is not an id, and changes when entry files move.` |
  > | T15 | C++ | `baked` id enum | `/// The key of an entry of <value>. Store or send ToWire(id): the underlying value is` / `/// a position, not an id, and changes when entry files move.` |
  > | T16 | Go | id `Position` | `// Position is the entry's place in <value> in this build, for array indexes in memory.` / `// It is not an id: it changes when entry files move.` |
  > | T17 | C++ | deleted `to_json`/`from_json` | `/// nlohmann/json would write the position: write ToWire(id), read <Id>FromWire(s).` |
  > | T18 | Go | `MarshalText`, `UnmarshalText` (ids; enums without `@codes`) | on an id, `// MarshalText writes the key, never the position.` and `// UnmarshalText reads a key; an unknown one is an error.`; on an enum, `// MarshalText writes the wire value, never the position.` and `// UnmarshalText reads a wire value; an unknown one is an error.` |
- CODEGEN.md §5.2's Go sample gains, after `func ToneMembers() iter.Seq[Tone]`, the two lines
  ```go
  func (self Tone) MarshalText() ([]byte, error)      // the wire value, for encoding/json and others
  func (self *Tone) UnmarshalText(text []byte) error  // a wire value; an unknown one is an error
  ```
  and, after the Go block, the sentence: "An enum without `@codes` has `MarshalText` and
  `UnmarshalText`, so text encoders write its wire value, never its declaration position; one with
  `@codes` has neither, its value being the code (DECISIONS 379)."
- CODEGEN.md §8.2 gains, after `canonList`, the helper (it lands in N-TS's commit, §7.2 S3, because
  `internal/gen/ts/spec_test.go` compares §8.2 with `runtime/helpers.ts.txt` byte for byte):
  ```ts
  /** Each key's position, for this file's own lookups and checks: positions are not ids, so no file exports one. */
  function canonIndex<K extends string>(members: ReadonlyArray<K>): Readonly<Record<K, number>> {
    const index = Object.create(null) as Record<K, number>;
    members.forEach((m, i) => { index[m] = i; });
    return Object.freeze(index);
  }
  ```
- CODEGEN.md §2.8: `:461` "(a `CanonMap`, an `<E>Index` table)" becomes "(a `CanonMap`, an enum's
  `<E>Index`, a table's `<Id>Members`)"; `:521` "through its `<E>Index`" becomes "through a private
  index this file builds from the owner's `<Id>Members` with `canonIndex` (§8.2)".
- CODEGEN.md §6.2's Go sample (`:1752-1760`) becomes the following (if N-GO's golden spells a line
  differently, the sample follows the golden, in N-GO's commit):
  ```go
  type StatusID struct{ position uint8 }

  var (
  	StatusIDOpen  = StatusID{0}
  	StatusIDTaken = StatusID{1}
  )

  func (self StatusID) String() string
  func (self StatusID) Position() int
  func (self StatusID) MarshalText() ([]byte, error)
  func (self *StatusID) UnmarshalText(text []byte) error
  func ParseStatusID(key string) (StatusID, bool)
  ```
- CODEGEN.md §8.3's sample (`:3626`) stays valid: `StatusIdIndex` is now that file's private const
  (`const StatusIdIndex = canonIndex(StatusIdMembers)`); where the sample shows its declaration, it
  becomes that line. CODEGEN.md §12 gains `| W8031 | warning | an entry position written to an
  output (§2.10) |`, in that table's columns.
- SPEC.md `:729`: "the keys of a public table form an id enum (Go `ItemID`, `StatusID`; C++ and
  TypeScript `ItemId`, `StatusId`) in `baked` and `embedded` modes." becomes "the keys of a public
  table form an id type (Go `ItemID`, `StatusID`, opaque: no integer converts to or from it; C++ an
  `enum class` `ItemId`, `StatusId`; TypeScript the union of the keys) in `baked` and `embedded` modes
  (§4.3: positions are not ids)."
- SPEC.md `:1692`: "`Get(id) *T` over the id enum" becomes "`Get(id) *T` over the id type".
- IMPLEMENTATION-PLAN §4.3: the `Position bool` field in the `Prov` sketch, right after `Kind`, with
  the comment `// an entry position's mark (EVALUATION.md §13.1); in the padding`.
- ERRORS.md: the E19xx heading becomes "E19xx, W19xx"; rows:
  > | W1912 | warning | check | EVALUATION.md §9.2 | an amend path that selects a table, keyed-list or map entry by position (`[#n]`) instead of by key (DECISIONS 379) |
  > | W8031 | warning | ir | CODEGEN.md §2.10 | an entry position, or a number or text made from one, written to an output: an emitted value, a stored export fn result or a `@text` file (DECISIONS 379) |
  > | W1912 | - | path:Name | `amend path {path} selects an entry by position, and the entry at a position changes when entries are added or entry files move: select it by key` |
  > | W8031 | - | output:Name | `a number or text made from an entry position is written to {output}; positions change when entry files move: write the entry's key or a @stable field, or wrap the position in positional(…) to write it on purpose` |
- DECISIONS:
  > 379. **Positions are not ids (SPEC §4.3, §15.2-§15.4; EVALUATION.md §9.2, §13.1; STDLIB.md §2.4;
  > CODEGEN.md §2.5, §2.8, §2.10, §5.3, §8.2; IMPLEMENTATION-PLAN §4.3; ERRORS.md W1912, W8031).**
  > Moving, renaming or adding an entry file changes entry order, so a position never stands for an
  > entry: its key does, and a number that must keep naming one is a `@stable` field or a `@codes`
  > code. By construction a `baked` table id is no integer: Go `struct{ position uintN }` with `var`
  > members, `String`, `MarshalText`/`UnmarshalText` (the key), `Parse<R>ID` and `Position()` (an
  > array index for one build, which generated code uses); C++ keeps `enum class` and deletes
  > `to_json`/`from_json`; TypeScript exports `<Id>Members` instead of `<Id>Index` and builds private
  > indexes with `canonIndex`. By detection, `enumerate()` and `indexOf` over entries mark the
  > position in its `Prov` (one bit, in padding); operators, templates, `Int`, `Float`, `String` and
  > `join` carry it; `positional(x)` drops it; stage E warns `W8031` where a marked value is written
  > to an output; `[#n]` amends on keyed collections warn `W1912`. Enums keep their numbers in
  > v0.1.9; Go enums without `@codes` gain `MarshalText`/`UnmarshalText`, which write the wire value
  > (their opacity, C++'s deleted nlohmann hooks for them and TypeScript's private `<E>Index` are
  > v0.2). Entry order still orders rows and iteration results; it never changes keys, fields,
  > refs, the lock or the fingerprint. Reason: N11 (handoff 2026-10-12 §4; log-2026-10-14, Louis:
  > "hard to fuck it up, like Rust"): Go's `encoding/json` and `database/sql` wrote a baked
  > `StatusID` as its position, nlohmann a C++ id enum's, and TypeScript exported `StatusIdIndex`; a
  > read-only scan of Resource and its consumers found no position persisted yet.

**Code.**
- `internal/value`: `prov.go` gains the field; new helpers `IsPosition(v)` (an Int, Float, Dur or Str
  whose `P.Position` is set) and `IsEntry(v)`.
- `internal/eval`: `reprov.go` adds `position bool` to `siteKey` and `(*Evaluator).positionProv(p)`
  (interns when `Via` and `Ext` are nil, copies otherwise; never writes `p`); `reprov` keeps the mark;
  `ops.go:31` (unary) and `:99-129` (binop), `strings.go:14-60` (templates), `call.go:306-329`
  (`runStd`: `Int`, `Float`, `String`, `join`) twin `p` when an operand is marked; `stdhost.go` adds
  `Position(p)` to `std.Host` (`std/std.go:14-34`); `std/seq.go:242-248` (`enumerate`) and `:98-101`
  (`indexOf`) use the twin when `IsEntry`; `std/math.go:11` adds `positional`; `memo_fp.go:237-250`
  hashes `Position`. Step costs are unchanged.
- `internal/check`: `builtins.go:177-199` and `constants.go` add `positional` (in N-EVAL, granted,
  so N-EVAL's tests can use it); `layers.go:132-134` reports `W1912` when `!plainList(t)` (N-CHECK).
- `internal/ir`: new `positions.go` (the walk, records visited once, in traversal order), called after
  `s.precompute()` in `stage.go:48`; the name plans: Go `goplan_scopes.go:226-246` adds `MarshalText`
  and `UnmarshalText` to the shared `goIDEnumMethods` list (reserved on every enum and id; `goEnumMethods`
  derives from it, `internal/ir/tables.go:197-199`) and `Position` to a new id-only list (enums do not
  reserve it); TS `tsplan.go:287-295` plans `<Id>Members` and the private foreign index names. A declared name colliding with a reserved one is the existing `E8005` with its
  existing way out.
- `internal/gen/go`: `enums.go:111-133` a new id writer, and point 16's two methods on every enum
  without `@codes`; `baked.go:46` and every index and int→id site
  (the Go compile in the golden tests, `golden_test.go:176-180`, lists them).
- `internal/gen/cpp`: `idenums.go:36-57` and `:81`.
- `internal/gen/ts`: `enums.go:183-216`, `fns.go:149-192`, `decode_value.go:306-322`,
  `runtime/helpers.ts.txt` (exactly §8.2's text).

**Tests** (all fail on `6a5fc5f3` unless marked guard):

| Package | Test | Proves | Rule |
|---|---|---|---|
| value | `TestProvSizeUnchanged` (guard) | `reflect.TypeFor[value.Prov]().Size() == 32` (no `unsafe`, even in tests: go.md §7, `internal/testkit/deps_test.go`) | 373, 379 |
| eval | `TestEntryPositions` | table, filtered-table and `keys()` `enumerate` and `indexOf` are marked; a plain list and `map(.name)` are not | EVALUATION §13.1 |
| eval | `TestPositionCarried` | each carrying rule; Bool, `at(i)`, `len`, `positional` do not carry | EVALUATION §13.1 |
| eval | `TestPositionInvisible` | Equal, hash and CanonText identical with and without the mark (explain is the API's and the CLI's, not eval's) | 379 |
| eval | `TestMemoFingerprintPosition` | memo fingerprints differ by the bit | ADR-0011 |
| build | `TestW8031ColdWarm` (in T-BUILD, after N-IR-W8031) | a `W8031` fixture gives the same findings cold and incremental | ADR-0011 |
| check | `findings/W1912_1..3.txtar` | table, keyed list, map warn; a plain-list case is accepted | EVALUATION §9.2 |
| ir | `findings/W8031_<n>.txtar` (one per case) | `i + 1` in a json value; a `@text` join; an `indexOf` field; a map key; a stored fn result; a cross-package site | CODEGEN §2.10 |
| ir | `W8031` accept cases | a plain list, a check-only use, `positional`, a view-only emit; 1,000 entries give one finding | CODEGEN §2.10 |
| gen/go | `TestIDTypeOpaque` | `json.Marshal` gives `{"S":"taken","L":["open"]}` (today `{"S":1,"L":"AA=="}`); an unmarshal round trip; `"nope"` is an error; sql and gob converters error; `StatusID(1)`, `= 1`, `uint8(id)`, `id+1` fail to compile (a new run-expect-compile-failure helper) | CODEGEN §5.3 |
| gen/go | `TestEnumMarshalText` | `json.Marshal` of a struct with a `Tone` field and of a `map[Tone]int` writes wire names (`"series-1"`); a round trip; an unknown wire value and a non-member value are errors; an `@codes` enum keeps its integer encoding (it has neither method) | CODEGEN §5.2 |
| gen/cpp | `TestIDEnumRefusesNlohmann` | with g++ and clang++: `json j = StatusId::open;` and `get<StatusId>()` fail to compile; `ToWire`/`FromWire` compile | CODEGEN §5.3 |
| gen/ts | `TestNoExportedIdIndex` | no golden matches `^export const \w+IdIndex` | CODEGEN §5.3 |
| gen/ts | drivers `baked.test.js:13`, `emptytable.test.js:3-7` | assert `<Id>Members` and the absence of `<Id>Index` | CODEGEN §5.3 |

**Goldens that change** (regenerated with `-update`, the diff read):
- Go: `examples/teamboard/.../teamboard.gen.go`; `examples/features/shared/.../go/{core,shared}`;
  `examples/features/dependent/.../dependent.gen.go`; `internal/gen/go/testdata/` (constructs,
  data/dependentdefinebaked, data/shop/demo/base, emberfall baked core and world, emberfall data core
  and world, escapes, keys, lookups, teamboard, translated/calc, unions/genunionbaked).
- C++: `examples/features/shared/.../core.gen.h`; `examples/resource/vocab/.../vocab.gen.h`;
  `internal/gen/cpp/testdata/golden/{baked, emberfall-core, emberfall-world-baked, empty-owner-b}`.
- TS: `examples/teamboard/expected/web/teamboard/generated.ts`;
  `examples/features/shared/.../ts/{core,shared}.ts`; the 15 `internal/gen/ts/testdata/ts/*.txtar`
  (baked, rowplain_baked, rowkeyed_baked, emptytable, ref_lookup_empty_data, stable_shapes,
  precomputed_table, rowplain_embedded, imports, shared_embedded, ref_lookup_data, shared_baked,
  stable_baked, shared_data, shared_types).
- Fixtures gaining `W1912`: `internal/eval/testdata/layers/{amend,order}.txtar`,
  `internal/eval/testdata/parity/fields.txtar`. No example `findings.txt` changes.
- Go, also: every Go golden holding an enum without `@codes` gains point 16's two methods; in those
  files nothing else may change beyond the lists above.
- A golden diff outside these lists is a bug to explain before the unit lands.

**Performance guard** (robustness first: the mark must cost nothing visible).
`BenchmarkCapturingPredicate` and `BenchmarkBuiltinsScale` keep their allocs/op; Resource `canon
check` (3 runs, interleaved A/B against v0.1.8 at equal load, DECISIONS 261) shows no wall or RSS
regression beyond noise (median within 3%). A measured regression beyond that does not narrow the
detection (robustness first): it is recorded, with its numbers, in the log and the reply, and it
blocks the release only under §8's rule (more than 10%).

**Units.** **N-EVAL** (M, opus; frozen contract; after S2): `internal/value`, `internal/eval`,
`internal/eval/std` (one owner, EVL; one commit), plus, granted, `positional`'s rows in
`internal/check/builtins.go` and `constants.go`. **N-CHECK** (S, opus: it changes `check`'s semantics;
after N-EVAL and Q-CHECK): `internal/check`, `W1912` only, plus, granted, the `-update` regeneration of
exactly `internal/eval/testdata/layers/{amend,order}.txtar` and `internal/eval/testdata/parity/
fields.txtar`, and `W1912`'s line in `internal/testkit/progen/testdata/gaps.txt`. **N-IR-PLANS** (XS,
sonnet): the name plans in `internal/ir`; before the generators. **N-IR-W8031** (S, sonnet, after
N-EVAL and N-IR-PLANS): `internal/ir` `positions.go`, plus `W8031`'s line in `gaps.txt`. **N-GO**
(M-L, sonnet, opus review; after N-IR-PLANS): `internal/gen/go`.
**N-CPP** (S, sonnet; after N-IR-PLANS): `internal/gen/cpp`. **N-TS** (M, sonnet; after N-IR-PLANS):
`internal/gen/ts`. Each generator unit regenerates its example goldens
(`internal/testkit/golden` `TestExamples`).

**Consumer migration** (for the reply, §8): Go: persist `id.String()` and read with `Parse<R>ID`;
`Position()` is for in-memory arrays only; a `const` use of a member breaks at compile time; JSON of
an id field changes from a number to the key, and numbers already stored stop unmarshalling (loudly):
convert stored data with the old build's mapping. C++: replace nlohmann on ids with `ToWire` /
`<Id>FromWire`. TS: use the key, or `<Id>Members.indexOf(id)` in memory.

## 4. Import tiers (DECISIONS 380)

**Facts.**
- The project grammar already accepts a map whose values are lists of dotted identifiers
  (GRAMMAR.md:1077-1084; `internal/syntax/parse_amend.go:123-170`); `...` is a token (GRAMMAR §2.8),
  so `a...` is no project value.
- Project keys are dispatched by `keyRules` (`internal/project/constants.go:37-61`) and read in
  `schema.go:17-91`; an error there fails `Load` and stops `Open` (API.md O3; `build/project.go:122`).
  `studio`'s package is checked after the scan by `project.CheckStudio` (`studio.go:8`), called from
  `build/check.go:209`, reporting `E1012` into the project's own bag, which every result includes
  (`pipeline.go:383,417`). `project.local.canon` holds only `roots`; another key is `E1014` `key`
  (`local.go:57`).
- Packages exist only under the project directory (TYPES §3.1; `project.Scan`); roots hold loaded and
  emitted files only. Imports are per file; layer and translation files hold none (GRAMMAR §5.2);
  `amend` names its own package only (EVALUATION §9.2); `load` reads non-Canon files; views use the
  studio package without an import (SPEC §16.11); there are no re-exports.
- `check` binds imports (`check/imports.go:35-63`, `E2003` at `f.Span(imp.Path)`) and reports cycles
  (`E2002`, `imports.go:128-164`). `E2001` (package `build`) is a structural rule run after phase 2
  over the loaded units (`build/dirs.go:16`, `pipeline.go:151`).
- The check key holds `project.canon`'s SHA-256 (`cachecheck.go:22-26,131-133`; `project.go:135`):
  any `project.canon` edit is a full check, never a Recheck. Recheck also requires identical import
  text (`check/recheck.go:146`). Bags sort and deduplicate (`diag/bag.go:35-38`).
- Free codes: `E1017`, `E1018`, `E2007`. `ProjectInfo` is Name, Doc, Roots (`api/canon.go:134-146`).
  The formatter lays project lists and maps out generically and never reorders (FORMATTER §11). The
  TextMate grammar highlights only the `project` keyword. The guide lists project keys in
  `internal/cli/guide/syntax.md:13-66` and common codes in `errors.md:36-79`.
- Harnesses: `internal/project/findings_test.go:80-106` (calls `CheckStudio` after parsing);
  `build/findings_test.go:19-28` (accepts `project.canon`, `select`, `check-only`); warm vs cold with
  `pair`/`same` (`build/incremental_internal_test.go:57-80`). The check harness cannot read
  `project.canon`. All examples share `examples/project.canon` (40 packages, 21 edges); the golden
  test checks `features.<name>` alone (`testkit/golden/examples_test.go:116`).
- Resource (read-only): 105 packages, 356 distinct package edges over 634 import lines; top
  directories import each other both ways (ai↔items, engine↔items), so tiers cannot follow top
  directories; the real layering is by role (every `*.vocab*` package imports only `header` or
  another vocab package). The declaration in the reply (§8) places all 105 packages with 0
  violations.

**Problem.** Acyclicity (`E2002`) lets an agent add an upward import that is not yet a cycle. A
project needs a declared direction every command enforces: general, one spelling, no knob, no grammar
or frozen-contract change, robust under the incremental machinery.

**Decision.**
1. **Form.** `tiers`: an ordered map from tier name to a list of package paths, lowest tier first.
   Names give readable messages and take a doc comment each; Canon maps are ordered and `fmt` never
   reorders. Rejected: a list of lists (messages would say "tier 2"); a per-package annotation
   (scattered, a grammar change); a list of records.
2. **Paths.** A dotted identifier places that package and every package below it; a longer listed
   path wins; a path need not be a package itself (`services` places `services.admin`). Same spelling
   as `studio:`; no grammar change. Rejected: exact paths only (every new package edits
   `project.canon`); the CLI's `a...` (a grammar and frozen syntax-tree change, or strings, a second
   spelling of a package path); wildcards (`*.vocab`, undefined precedence); regular expressions.
3. **Direction.** A package may import its own tier and every earlier tier. Rejected: adjacent-only
   layering, sibling independence, a graph of tiers.
4. **Strict coverage.** Once a tier is declared, every package must be placed (`E1018` `unplaced`): an
   unplaced package would let a low tier reach a high one through it.
5. **A path placing no package is an error** (`E1018` `unused`), as `E1012`: a moved or deleted package
   must not silently change the architecture.
6. **Overlaps.** The same path twice anywhere is `E1017` `duplicate`; an ancestor and a descendant in
   different tiers is not an error (the longer path wins).
7. **Roots.** Not applicable: packages live only under the project directory.
8. **Edges.** Only `import` lines count, each judged at its path span; every chain then goes down or
   across, so no transitive check is needed. Not counted: layer files (no imports), `load`, a view's
   implicit use of the studio package (an explicit `import studio` is judged), emit copies and
   consumer roots (`E8004` covers copies).
9. **No second finding.** An import of no package (`E2003`), or from or to an unplaced package
   (`E1018`), is not judged (DECISIONS 141).
10. **`project.local.canon`** cannot set `tiers` (already `E1014` `key`): a package's verdict is the
    same on every machine (332).
11. **Layers, `--layer`, `--only-root`:** no interaction. `E2007` blocks writes like any error (343);
    it breaks no declaration, so values are still evaluated.
12. **Incremental:** nothing new. `E1018` is recomputed at every snapshot's load, `E2007` at every
    analysis; `project.canon` is part of the check key; the language server reads unsaved buffers.
13. **Codes.** Shape errors stop `Open`: `E1005` `tier`, `E1006` `tiers`, `E1017`. Placement errors
    are findings (exit 1): `E1018`, in the project's own bag, reported by every command whatever it
    selects: `unplaced` at the `tiers` key, one finding per package in package-name order; `unused`
    at the listed path's span. Import errors: `E2007`, at the import's path span. Rejected: one code for all (mixes stopping and non-stopping);
    a related note into `project.canon`.
14. **Where.** Package `project` owns `E1017`, `E1018`, `E2007`; `build` calls `CheckTiers` in `load`
    (beside `CheckStudio`) and `CheckTierImports` in `run.check` (after `dirConflicts`). The checker,
    its journal and Recheck stay untouched (`E1012` and `E2001` are the precedents); no frozen
    contract changes. Rejected: checking in `check.bindImport` (it touches the journal, and the check
    harness cannot read `project.canon`).
15. **Surfaces.** No change to the API, `ProjectInfo`, `PackageInfo`, the view model, `canon fmt` or
    the TextMate grammar; the language server publishes the findings as today; the guide gets the
    rule.
16. **No knobs:** no exception list, no modes.

**Code.**
- `internal/project/tiers.go` (new), plus `project.go` (field `Tiers Tiers`), `constants.go`
  (`keyTiers = "tiers"` and its `keyRules` entry), a line in `doc.go`:
```go
type Tiers struct { List []Tier; Span source.Span /* the key */; at map[string]int }
type Tier struct { Name string; Span source.Span; Paths []Package }
func (t Tiers) Declared() bool
func (t Tiers) Of(pkg string) (tier int, path string, ok bool) // pkg, then cut at each last "."; zero value safe
func (s *schema) tiers(e *syntax.ProjectEntry)
func CheckTiers(p *Project, units []*Unit, bag *diag.Bag)                          // E1018
func CheckTierImports(p *Project, units, all []*Unit, bagOf func(string) *diag.Bag) // E2007
```
- `schema.tiers`: not a map → `E1006` `tiers`; per entry (skip it if `isBad`): a name that is not an
  identifier → `E1017` `name`, a name seen before → `E1005` `tier`, a value that is not a non-empty
  list → `E1006` `tiers`; per item (skip it if `isBad`): not a `QualifiedName` → `E1006` `tiers`, a
  path seen before → `E1017` `duplicate` with the first span.
- `Of` cuts only at segment boundaries (`items` never places `itemsx`).
- `CheckTierImports`: for each `f.Imports` entry, skip a nil path and a target not in `all` (binary
  search: `all` is in name order). Range over slices only (the audit's `maprange`). About
  O(imports × path segments).
- `internal/build/check.go:209`: `project.CheckTiers(s.proj, s.units, s.own)`;
  `internal/build/pipeline.go:151`: `project.CheckTierImports(r.s.proj, r.loaded, r.s.units, r.s.bag)`.
- The project findings harness (`internal/project/findings_test.go`) calls both after `CheckStudio`.

**Spec text.**
- GRAMMAR.md §7.1, a new last row:
  > | `tiers` | ordered map tier name → list of package paths | `{}` (none) | the allowed import
  > directions (TYPES.md §3.1, DECISIONS 380), tiers written lowest first. Names are `IDENT`s (`E1017`
  > `name`), unique (`E1005` `tier`). A value is a non-empty list of package paths written as dotted
  > identifiers (`[items.vocab, header]`); another value, an empty list or another item is `E1006`
  > `tiers`. A path is listed once over all tiers (`E1017` `duplicate`). A path **places** the
  > package of that name and every package below it, except one a longer listed path places; it need
  > not name a package. Once a tier is declared, a package no path places and a path that places no
  > package are `E1018`, found after the scan like `E1012`. Not allowed in `project.local.canon`
  > (`E1014`) |
- GRAMMAR.md §12: `E1005`'s trigger becomes "a key, root name or tier name given twice"; new rows
  `| E1017 | error | a tiers name that is not an identifier, or a package path listed twice (§7.1) |`
  and `| E1018 | error | with tiers: a package no listed path places, or a path that places none
  (§7.1) |`.
- TYPES.md §3.1, a new bullet after "Imports are acyclic":
  > - **Tiers** (DECISIONS 380). When `project.canon` declares `tiers` (GRAMMAR.md §7.1), a package
  > belongs to the tier of the path that places it and may import packages of its own tier and of the
  > tiers written before it; any other `import` is `E2007`, at its path. Only `import` lines count:
  > layer and translation files hold none, `load` reads files, and a view's use of the studio package
  > needs no import (SPEC §16.11); an explicit one is judged. An import of no package (`E2003`), or
  > from or of a package no tier places (`E1018`), is not judged.
- TYPES.md diagnostics: `| E2007 | error | an import of a package of a later tier (§3.1) |`.
- SPEC.md §3.1, a new table row, written in the style of the `languages` row (each cell's type in its
  own code span, on one line):
  > | `tiers` | `{identifier: [package path](1..)}`, in order | `{}` | allowed import directions, lowest tier first (§3.2); `project.local.canon` cannot set it (`E1014`) |
- SPEC.md §3.2, a new bullet:
  > - With `tiers` in `project.canon` (DECISIONS 380; spec/GRAMMAR.md §7.1), a listed path places its
  > package and every package below it, a longer path winning, and every package must be placed. A
  > package imports only from its own tier and the tiers listed before it (`E2007`):
  > `tiers { ids: [game.vocab], data: [game] }` lets `game.items` import `game.vocab`, never the
  > reverse.
- ERRORS.md, code rows (and `E1005`'s meaning becomes "a project key, root name or tier name given
  twice"):
  > | E1017 | error | project | GRAMMAR.md §7.1 | a `tiers` name that is not an identifier, or a
  > package path listed twice in `tiers` (DECISIONS 380) |
  > | E1018 | error | project | GRAMMAR.md §7.1 | with `tiers` declared, a package no listed path
  > places, or a listed path that places no package (DECISIONS 380) |
  > | E2007 | error | project | TYPES.md §3.1 | an import of a package `tiers` places in a later tier
  > than the importing package's (DECISIONS 380) |
- ERRORS.md, message rows:
  > | E1005 | tier | name:Name | `duplicate tier "{name}" in project.canon` |
  > | E1006 | tiers | - | `"tiers" must be a map of tier names to non-empty lists of package paths` |
  > | E1017 | name | name:Name | `invalid tier name "{name}": a tier name is an identifier` |
  > | E1017 | duplicate | path:Name, first:Loc | `tiers: {path} is listed twice (first at {first})` |
  > | E1018 | unplaced | pkg:Name | `tiers: no tier places package {pkg}; list it, or a package path above it, in a tier` |
  > | E1018 | unused | path:Name | `tiers: {path} places no package; remove it or correct it` |
  > | E2007 | - | pkg:Name, tier:Name, imported:Name, importedTier:Name | `{pkg} (tier {tier}) cannot import {imported} (tier {importedTier}): a package imports only from its own tier and the tiers listed before it in project.canon` |
- IMPLEMENTATION-PLAN §7.9, a new row: `| tiers | tiers in project.canon: a longer path placing a
  sub-package in an earlier tier, a downward import | v0.1.9 | present |`.
- DECISIONS:
  > 380. **Import tiers (GRAMMAR.md §7.1; TYPES.md §3.1; SPEC §3.1, §3.2; ERRORS.md E1005 `tier`,
  > E1006 `tiers`, E1017, E1018, E2007).** `project.canon` takes `tiers`, an ordered map of tier names
  > to lists of package paths, lowest tier first (default `{}`: no tiers). A listed path places the
  > package of that name and every package below it, except one a longer listed path places; it need
  > not be a package (`services` places `services.admin`). An `import` of a package placed in a tier
  > written after the importer's is `E2007` at the import, naming both packages and tiers; the same
  > tier and every earlier tier are allowed. Only `import` lines count, each judged alone: when every
  > import goes to the same or an earlier tier, so does every chain. Once a tier is declared, a
  > package no path places, and a path that places no package, are `E1018` in the project's bag,
  > reported by every command whatever it selects (as `E1012`); an import of no package (`E2003`), or
  > from or of an unplaced package, is not judged. Tier names are identifiers (`E1017` `name`),
  > unique (`E1005` `tier`); a value is a non-empty list of dotted identifiers (`E1006` `tiers`); a
  > path is listed once over all tiers (`E1017` `duplicate`); these stop `Open` (API.md O3).
  > `project.local.canon` cannot set `tiers` (`E1014`, 332). Package `project` reports all three
  > codes; the checker and Recheck are untouched, and a `project.canon` change is never a Recheck.
  > No API change. Refused: a graph of tiers, adjacent-only layering, per-import exceptions,
  > wildcards, string patterns and an `a...` suffix, free unplaced packages. Reason: Louis,
  > 2026-10-14 ("extremely important"; handoff 2026-10-12 N8): acyclicity lets an agent add an upward
  > import that is not yet a cycle; Resource's rule "pure helpers < `.vocab` (ids) < theme data <
  > `client`/`clientui` projections. Imports only go down." had no check.

**Tests.**

| Test | Proves |
|---|---|
| `internal/project/testdata/findings/E1005_<n>.txtar` | a duplicate tier name |
| `E1006_<n>.txtar` (two files) | `tiers` given as a list; a value that is not a list, an empty list, a string item |
| `E1017_1.txtar` | a string name and a reserved-word name |
| `E1017_2.txtar` | a path in two tiers, and twice in one tier, each with "first at" |
| `E1018_1.txtar` | an unplaced package; its upward import gives no `E2007` |
| `E1018_2.txtar` | a typo path; a path whose packages are all placed by longer paths |
| `E2007_1.txtar` | carve-out `a.vocab` (ids) importing `a` (data); same-tier and two-tiers-down imports clean; two upward lines give two findings |
| `clean_<n>.txtar` (two files) | a path that is not a package, a three-level carve-out; `tiers {}` with an "upward" import gives nothing |
| `internal/build/testdata/findings/E2007_1.txtar` (`check-only`, `select a`) | the wiring: `E2007` in the selected package, `E1018` of an unselected package still reported |
| `internal/build/tiers_internal_test.go` `TestTiersFollowProjectCanon` with `testdata/incremental/tiers.txtar` (cites IMPLEMENTATION-PLAN §7.6, API.md S2) | with `pair`/`same`: `E2007` present; still present after an entry-value edit (the Recheck path); gone after a `project.canon` edit moving the package |

`<n>` is the next free number in each code's sequence.

**Examples.** `examples/project.canon` gains (each tier with a `///` doc comment):
```
tiers {
  shared: [sovcommon]
  studio: [studio]
  ids: [features.tiers.ids, resource.vocab]
  data: [balance, features, game, guide, pipeline, resource, service, teamboard, telemetry]
}
```
and a new example `examples/features/tiers/`: `tiers.canon` (package `features.tiers` importing
`features.tiers.ids { Rarity }`, a documented record and table), `ids/ids.canon` (a documented enum),
`expected/findings.txt` = `0 errors, 0 warnings in 1 package (…)` written by the compiler. The block
must give no finding on every example; if `canon check` reports `E1018` or `E2007` on it, the unit
adjusts paths minimally to the real graph (never by adding a tier) and logs it. `examples/project.canon`
must be a `canon fmt` fixed point; every example golden is regenerated with `-update` and its diff
read (no finding change is expected).

**Guide.** Written by the GUIDE unit (§7.2). `internal/cli/guide/syntax.md`: an "Import tiers" section with
this block (one entry per line: a one-line map without commas is `E1117`)
```
tiers {
  ids: [shop.vocab]
  data: [shop]
}
```
and three bullets (placement and the longest path; `E1018` and
`E2007` with their fixes; only `import` lines count, and `E1014` in `project.local.canon`).
`errors.md`, two rows: `E2007` "an import climbs `tiers`: import the same or an earlier tier, move the
code down, or change `tiers`"; `E1017` `E1018` "`tiers`: a name that is not an identifier, a path
listed twice; a package no path places, or a path that places none". Every recipe passes
`canon check` in an examples project.

**Units.** **T-PROJECT** (M, opus): `internal/project` (`tiers.go`, `schema.go`, `project.go`,
`constants.go`, `doc.go`, the harness, the txtars), plus the lines of `E1017`, `E1018` and `E2007`
in `internal/testkit/progen/testdata/gaps.txt` (§7.2, S3). **T-BUILD** (S, sonnet, after T-PROJECT,
ASSETS and N-IR-W8031): the two call lines, the two build tests, and §3's `TestW8031ColdWarm`
(`internal/build`). **T-EXAMPLES** (S, sonnet, after T-BUILD, Q-SYNTAX, Q-EDITORS, N-GO, N-CPP,
N-TS): `examples/project.canon` and `examples/features/tiers/`. It creates the empty discovery seed
`examples/features/tiers/expected/MANIFEST` (the harness finds examples by it,
`internal/testkit/golden/examples_test.go:80-87`), then runs `go test ./internal/testkit/golden -run
TestExamples -update`, which writes that MANIFEST and `findings.txt` and regenerates every example
golden (the diff is read: only the new example's files may change). Granted, for the files it adds
or changes only: the `-update` regeneration of their AST goldens (`internal/syntax/testdata/ast/`)
and of their scope snapshots (`internal/testkit/vscodegrammar`, `scopes_test.go`). The guide text is
the GUIDE unit's (§7.2).

## 5. M4 call (a): dot-prefixed files are not sources (DECISIONS 381; amends 332)

**Facts.**
- One predicate decides what is a source: `project.IsSource` (`internal/project/scan.go:47`). It feeds
  `Scan` (`scan.go:15`), the S5 per-directory key (`internal/workspace/snapfs.go:351`) and
  `movedSources` (`internal/build/reread.go:101`). `Scan`'s callers: `build/project.go:87`,
  `build/check.go:77`, `build/reads.go:84`, `cli/listings.go:14`, `cli/fmt.go:210`,
  `workspace/evaluate.go:109`, `api/watch_reads.go:221`: one change reaches them all.
- Today: API.md O2 skips dot-prefixed directories only. A dangling Emacs lock `a/.#a.canon` makes every
  `canon check` exit 2 (`canon: a/.#a.canon: open: no such file or directory`); `.hidden.canon` is a
  source (`fmt --check` lists it); a file named exactly `.canon` is one too.
- Walks that keep their own rules: `load.dir` (`internal/load/walk.go`; WIRE.md:652-653 "A name
  starting with `.` matches only a pattern segment that starts with a literal `.`. `**` never enters
  a directory whose name starts with `.`"; SPEC.md:1433); `@files` placement (API N2/N3,
  `edit/place.go:192` and the journal already refuse a `.` segment, so no edit can create a file the
  scan would drop); `.canon/` (journal, writer locks); the stray scan (`build/strays.go:121` already
  skips `.` names, 356); asset listings (`hosts.go:344`), staging, link and case listings
  (`stage.go:171`, `nolinks.go:78`, `realpaths.go:95`, `textstale.go:172`) and the edit's
  empty-directory pruning (`applywork.go:449`, `commit.go:426`) keep seeing dot files;
  `project.Find` (`find.go:63`); the workspace and LSP listing plumbing.
- No dot-prefixed `.canon` file is tracked in Resource, Source, the services or this repository.
  `canon new .x` is already refused (identifier rule).
- Tests encoding the old rule: `internal/project/scan_test.go:23` (`TestIsSource`, comment :11-13),
  `internal/workspace/stale_test.go:102` (`TestStaleAddedFile`, comment :93-95),
  `api/edit_hidden_test.go:55-56` (`TestEditHiddenSource`, the `c/.h.canon` case). API.md §7.2's
  `format` row says "`.canon` files included".

**Decision.**
1. `IsSource` adds `!strings.HasPrefix(path.Base(name), hiddenPrefix)` (the constant exists), using
   `path.Base(name)`, never `e.Name()` (`TestIsSource` passes a stub entry named "f"); the `Scan` and
   `IsSource` doc comments say so. One shared predicate: the scan, the S5 key and `movedSources` can
   never disagree. Rejected: filtering in `scanDir` only (they would disagree); a `.#`-only rule
   (Louis said dot-prefixed files).
2. An explicitly named dot file gets no exception: `canon check a/.hidden.canon` and
   `canon fmt a/.hidden.canon` print `canon: unknown package: a/.hidden.canon` and exit 2, through the
   existing `Match` (`internal/project/selectors.go:69`); `fmt` with no path leaves it alone. Rejected:
   explicit arguments bypassing the scan (two notions of "source": fmt would touch what check ignores).
3. No finding or warning for a skipped dot file: lock files are transient, Resource's gates run
   `--max-warnings 0`, and dot directories give none either.
4. The language server: no code change. An open dot file is assigned to its project, which does not
   scan it, so it gets no diagnostics (as a file under `.git/`); formatting still works on the buffer.
   Rejected: loose syntax diagnostics for skipped files (new semantics).
5. `project.canon` and `project.local.canon` keep their own rule (skipped by name at the top).

**Spec text.**
- API.md O2 (`spec/API.md:102-106`): the parenthetical describing the scanned file set becomes "(every
  `.canon` file under the project root except `project.canon` and `project.local.canon` at its top,
  skipping every directory and every file whose name starts with `.`, and the directory listing needed
  to map packages to directories)", and this sentence follows it: "A skipped name is nobody's source:
  it is not parsed, it is in a read set (S3, S5) only as a file a `load` names, it is no event's file
  (W13), and a selector that names it is an unknown package (CLI.md §2.2). No finding is given for
  it." The rest of O2 is unchanged.
- API.md §7.2, the `format` row: ", `.canon` files included" becomes "(only a loaded file can: the scan
  reads no `.canon` file or directory so named, O2)".
- API.md S5 (`:240`): "swap file" becomes "swap or lock file". W13 (`:1626`): "editor swap files,
  `.canon/`" becomes "editor swap and lock files, names starting with `.` (`.canon/` among them)".
- CLI.md §2.2, appended: "A file whose name starts with `.` is no source (API.md O2); naming it is an
  unknown package, exit 2."
- SPEC.md §3.3: no change (it states no scan rule; API.md O2 owns it).
- DECISIONS: 332 gets "[amended by 381]", and:
  > 381. **Dot-prefixed files are not sources (API.md O2, S5, W13, §7.2; CLI.md §2.2; amends 332).**
  > The scan skips every file whose name starts with `.`, as it already skipped such directories:
  > `.hidden.canon`, an editor lock file `.#a.canon`, a macOS `._a.canon` and a file named `.canon`
  > are no package's sources. One predicate decides for the scan, the S5 listing key and the build's
  > re-read, so they never disagree. A skipped file is nobody's source: not parsed, in a read set only
  > as a file a `load` names, no event's file; a selector naming it is an unknown package (exit 2),
  > explicit arguments included; no finding names it. `load.dir` (WIRE.md §6.5's dotfile rule),
  > `@files` placement (a `.` segment is refused, API.md N2, N3), `.canon/`, the stray scan (356) and
  > asset listings keep their own rules. Reason: Louis, 2026-10-14 (M4 call (a), handoff
  > 2026-10-01-m4-complete): an editor's lock file was a source, and a dangling Emacs lock
  > `a/.#a.canon` made every `canon check` exit 2.

**Tests.**
- `internal/project/scan_test.go`: `a/.hidden.canon` flips to false; add `a/.#a.canon`, `.top.canon`,
  `a/.canon` (false) and `a/x.canon` (true).
- `internal/project/sources_test.go`: dot files join the txtar tree; `TestScan` and
  `TestReaderGroupsByPackage` fail first.
- `internal/workspace/stale_test.go`: the `.hidden.canon` and `.#a.canon` cases flip to false.
- `api/edit_hidden_test.go`: `c:m` becomes `ErrNoPath`. `api/snapshot_test.go`: new
  `TestRevisionIgnoresDotFiles`. `api/watch_test.go`: a lock file and a hidden source give no event.
- `internal/cli/testdata/commands/`: four new txtars, `check_dotfile`, `check_dotfile_all`,
  `fmt_dotfile`, `fmt_dotfile_named` (point 2's message and exit code).

**Unit O2** (S, sonnet): `internal/project` (the code and its tests) and, because the old rule is
encoded there, exactly the test files named above in `internal/workspace`, `api` and
`internal/cli/testdata/commands`. Nothing else.

## 6. M4 call (c): a target for the studio's refresh after an edit (DECISIONS 382; amends 261)

**Facts.**
- IMPLEMENTATION-PLAN §7.6's table (`:1087`) gates Edit p95 ≤ 300 ms and Evaluate p95 ≤ 150 ms. Its
  `:1094-1097` and DECISIONS 261 say the bench "calls `Value` before each timed `Set`, as the studio
  refreshes its view model after a committed edit (API.md V14); that refresh is outside NFR-01's gated
  measures".
- The harness is `internal/testkit/cmd/benchgen` (`BENCH_EDIT_N=7000`): `draw` calls `Value` of a random
  root, untimed; `set` times the Edit, then the Evaluate of the entry.
- Recorded numbers: M4 final (`f53ba0c`, quiet, load 0.7-1.8): Edit p95 0.267 s, Evaluate p95 15 ms.
  P18 (2026-09-30, load 6-10), `Value` after each Set: p50 448 ms, p95 671 ms; "240-470 ms" is the p50
  range over runs. v0.1.5 and v0.1.6 (load 2-10): Edit 0.326 / 0.322 s, Evaluate 0.242 / 0.241 s,
  both above their targets (recorded misses). Nothing recorded for v0.1.7 and v0.1.8.
- The first reader of an edited snapshot pays the every-package analysis: on a small project, Evaluate
  paid a checker run and the next `Value` paid none (one scratch run). The cost moved from the untimed
  `Value` into Evaluate.
- The studio, from its source (read-only): `refreshAll` calls `reloadVMs` (a `ViewModel` per package)
  and `reloadFindings` (`Check`); its server calls `ViewModel` (`hub.go:319`), `Value` (`rows.go:488`),
  `Evaluate` (`rows.go:568`).

**Decision.**
1. **The measure** is, per accepted Set, the Evaluate of the entry plus a `Value` of the edited entry
   right after it; their p95 over the run's Sets is the "refresh". Whichever call first needs the new
   snapshot's analysis pays it, so only the sum is stable. It is measured wherever `TestBenchEdit`
   measures Edit and Evaluate today, and consumes no random draw (the Set sequence is unchanged).
   Rejected: timing the next `draw`'s `Value` alone (it now reads about 0).
2. **The target** is refresh p95 ≤ 500 ms, unless the baseline is above 400 ms: then it is 1.25 × the
   baseline rounded up to the next 50 ms. The baseline is the median (with two runs, the mean) of the
   refresh p95 of the two "A" runs of §8's A/B (v0.1.8 with VMBENCH's test files), on the reference
   machine (DECISIONS 261), load recorded at start and end. This is the one number of this ADR set by
   a measurement, by this fixed rule; S1 writes 500 ms, and §8 corrects IMPLEMENTATION-PLAN §7.6,
   DECISIONS 382 and VMBENCH's constant before the tag if the rule gives another number.
3. **Not gated:** the studio's full reload (a `ViewModel` per package plus `Check`): its cost is unknown
   and it would add minutes to every bench run. §9 records it.
4. **A miss** never licenses a new cache, reuse or parallel stage in v0.1.9 (robustness first). It
   blocks the release only as a regression: median(B1, B2) > 1.10 × median(A1, A2), the runs going A,
   B, A, B (261). A pair whose start loads differ by more than 1.0 is re-run, at most three times;
   then the last pair counts and both loads are recorded. Otherwise a miss of the target is recorded
   in the log and the reply, as v0.1.5 and v0.1.6 recorded theirs. Cold check and RSS keep their
   absolute gates.
5. A lead found here, not acted on: `evalAnalysis` (`api/evaluate.go:145-158`) never calls
   `workspace.KeptSelection` (only `api/watch.go:195` does). It is performance work: parked (§9).

**Spec text.**
- IMPLEMENTATION-PLAN §7.6, a new table row: `| studio refresh after a committed edit (Evaluate of
  the entry, then a Value), p95 | ≤ 500 ms |` (corrected by §8 only if point 2's rule gives another
  number).
- IMPLEMENTATION-PLAN §7.6 `:1094-1097`, the sentence becomes: "After each timed `Set` it times the
  studio's next reads of the new snapshot (API.md V14), the `Evaluate` above then a `Value` of the
  edited entry; their sum is the refresh, gated at its own target (DECISIONS 382), since whichever
  first needs the snapshot's every-package analysis pays for it."
- The Makefile comment above `bench-edit` (`:242-244`) adds "refresh p95" to what it reports.
- DECISIONS: 261 gets "[amended by 382]", and:
  > 382. **A target for the studio's refresh after a committed edit (IMPLEMENTATION-PLAN §7.6; amends
  > 261).** NFR-01's edit bench times, after each timed `Set`, the `Evaluate` of the edited entry and
  > then a `Value` of it; their sum is the refresh, gated at p95 ≤ 500 ms on the reference machine.
  > Whichever call first needs the new snapshot's every-package analysis pays it, so only the sum is
  > stable. The studio's full reload (a `ViewModel` per package plus `Check`) is not gated. A miss
  > never licenses a cache, a reuse or a parallel stage (log-2026-10-14, robustness first); it blocks
  > a release only as a regression: the median of two runs more than 10% above the previous
  > release's, the runs interleaved at equal start load. Reason: Louis, 2026-10-14 (M4 call
  > (c)): the refresh cost 240-470 ms (P18) and nothing gated it (261).

**Unit VMBENCH** (XS, sonnet): `internal/testkit/cmd/benchgen` (its test files): after each timed `Set`,
time the `Value` of the edited entry right after the Evaluate already timed; report "refresh p95"
beside Edit and Evaluate; assert it against a `refreshTarget` constant set to 500 ms until point 2's
number replaces it. `go vet` and one short run (`BENCH_EDIT_N=200`) prove it; the real runs are §8's.

## 7. The run

### 7.1 Before starting

- Read: CLAUDE.md, `meta/state.md`, `meta/plan.md` (v0.1.9), this ADR whole, DOCTRINE §2-§5,
  `.claude/rules/orchestration.md`, `.claude/rules/go.md`.
- Base: `main` at or after the commit holding this ADR (`v0.1.8` is tagged and pushed).
- **Calls already made, never re-asked:** every decision of this ADR; technical calls are the
  orchestrator's (decided and logged in `meta/decisions/log-<date>.md`); only a philosophy question
  goes to Louis, in a file under `meta/handoff/`.
- **Rules and lessons** (from the v0.1.6 run, `meta/handoff/2026-10-12-v016-start-prompt.md`):
  - every agent `go test` or probe runs as `systemd-run --user --scope -q -p MemoryMax=3G -p
    MemorySwapMax=0 env GOTOOLCHAIN=local go test -timeout 300s ...`, and that command is in every
    VERIFY line; the orchestrator's `make check` runs under `MemoryMax=6G`; temp dirs go in `/var/tmp`;
  - at most 6 builders run at once, on disjoint packages, one builder per package at a time; at most
    2 of them run heavy tests (eval, `-race`, fuzz, the golden compile of generated code) at once;
  - never `git stash`, `git checkout --` or `git reset`; each unit works in a worktree
    (`isolation: "worktree"`), fast-forwarded to `main` first and reporting its base SHA; the
    orchestrator commits in the worktree and cherry-picks onto `main` (never merges), with
    `make check` before every `cherry-pick --continue`; meta commits go on `main` only;
  - read-only outside this directory (Resource, Source, Scripts, the services): a need there is a
    handoff;
  - check `ListAgents` before telling Louis units run; end every delegation prompt with "send your
    final report; do not stop without it";
  - concurrency tests also run at `GOMAXPROCS=2` and `4`;
  - every non-trivial diff gets `spec-reviewer` (opus); on a FAIL, resume the SAME builder with the
    numbered findings, then the SAME reviewer on the fixes only; a sonnet builder failing review twice
    is replaced by an opus one;
  - long gates (stress, `fuzz-edit`, the benches) run once, at the end (§8); builders run their own
    package tests;
  - status for Louis goes in `meta/handoff/<date>-v019-status.md`, with a two-line chat summary.

### 7.2 Steps

**S1, spec sync** (orchestrator; one commit `docs(spec): v0.1.9 decisions 374-382 (ADR-0021)`):
- every "Spec text" block of §2 to §6 except the ERRORS.md rows, verbatim;
- DECISIONS 374-382 appended in order; "[amended by 375]" on 230 and 352, "[amended by 377]" on 357,
  "[amended by 378]" on 366, "[amended by 381]" on 332, "[amended by 382]" on 261;
- ADR-0010's amendment and status line (§2.2);
- IMPLEMENTATION-PLAN §4.3 (the `Prov` field, §3 point 15), §4.5 (the `NoHook` comment, §2.5), §7.6
  (§6's row and sentence, with 500 ms; §8 corrects the number if §6's rule changes it), §7.9 (§4's
  row);
- meta: the handoff README (§2.7's correction), the plan and state's focus already point here (done
  with this ADR's commit); `meta/state.md` moves `NoHook` from "Owed, small" to a documented limit and
  adds §9's items to "Owed, small", staying at 80 lines or fewer;
- held back from S1: a spec block that a test compares byte for byte with code lands, written by the
  orchestrator, in the commit of the unit that changes that code. Known: CODEGEN §8.2's `canonIndex`
  helper (`internal/gen/ts/spec_test.go`, `TestHelperBlockIsTheSpecs`), in N-TS's commit. If S1's
  `make check` fails on another such test, hold that block back the same way, to the unit that
  changes the compared code, and log it;
- before the commit, `spec-reviewer` (opus) reviews S1's diff against this ADR: every block verbatim,
  nothing added;
- `make check` green. If `diag-check`, or a test comparing a document's code table with ERRORS.md,
  fails (diaggen also reads IMPLEMENTATION-PLAN), run S2 first and commit S1 and S2 together.

**S2, DIAG** (orchestrator, then one sonnet go-dev in the main tree; nothing else runs yet; one commit
`feat(diag): v0.1.9 codes (DECISIONS 376-380)`):
- the orchestrator writes every ERRORS.md row of §2.3 (`E3706`), §2.4 (`E8152` meaning, `file` and
  `dirCase`), §2.6 (`E1106` meaning, `E1138`), §3 (the E19xx heading, `W1912`, `W8031`) and §4
  (`E1005` meaning and `tier`, `E1006` `tiers`, `E1017`, `E1018`, `E2007`), and the count sentence:
  "The catalogue holds 331 codes: 302 errors, 26 warnings and 3 run-time codes, with 553 messages."
  diaggen checks the sentence; if it disagrees, recount (diaggen wins, never edit around it);
- the go-dev runs `go run ./internal/diag/cmd/diaggen -out internal/diag -runtime internal/gen`
  (`codes.go` and `codes_test.go` are regenerated, never hand-edited), fixes only the compile and test
  failures the regenerated registry forces (`E8152`'s bare variant becomes `file`: the two `E8152.At`
  calls in `internal/build/collide.go` become `E8152.AtFile`; `internal/diag/testdata/messages.txtar`
  follows through `go test ./internal/diag -run TestEveryMessageGolden -update`, and any message
  sample with it). It names no new code in production code (the `diag-code-untested` ratchet: §2.3
  point 8);
- `make check` green (`diag-code-unreported` only observes, `tools/audit/internal/rules/rules.go:113`,
  so codes nothing reports yet do not fail it). No review: no decision is left in it.

**S3, the units.** Start any unit whose "after" units have landed on `main`, within §7.1's caps. Each
delegation uses orchestration.md's template: GOAL from the unit's section; OWNER/SCOPE = exactly the
packages below (a granted exception is named); READ FIRST = this ADR's section, the spec sections it
cites, DOCTRINE §3-§5, `.claude/rules/go.md`; CONTRACTS = the frozen interfaces it reads (only N-EVAL
changes one, under §4's review rule; NOHOOK changes a comment); TESTS = the section's table (each test
cites its rule; every new code has its txtar); VERIFY = the capped `go test` of its packages,
`-race` where it touches concurrency, the log attached (builders do not run `make check`: the
orchestrator runs it under 6G before every cherry-pick, §7.1); REPORT BACK = files touched, gaps,
any choice not in this ADR (none is expected: an unexpected one is logged by the orchestrator under
§0's fallback), what is unverified. The prompt states the tier and why.

**The progen coverage ratchet.** `internal/testkit/progen/coverage_test.go` (in `go test ./...`)
fails on a code the compiler can report that has neither a mutation operator nor a line in
`internal/testkit/progen/testdata/gaps.txt`. The unit that first makes a new code reportable adds,
granted, its one line there, in the file's format (`CODE reason`), e.g. `E3706 an asset reached only
through a symbolic link outside the roots and the project (TYPES.md §13.4, DECISIONS 376), a fact of
the machine; internal/verify's E3706_*.txtar produce it` (precedent: E8026, E8027). The units:
Q-SYNTAX (`E1138`), ASSETS (`E3706`), T-PROJECT (`E1017`, `E1018`, `E2007`), N-CHECK (`W1912`),
N-IR-W8031 (`W8031`). A new variant of an existing code (`E8152` `dirCase`, `E1005` `tier`, `E1006`
`tiers`) needs no line.

| Unit | Packages (exclusive) | Tier | Size | After | § |
|---|---|---|---|---|---|
| STAMP | `internal/workspace`, `api/refresh_test.go` | sonnet | S | S2 | 2.1 |
| O2 | `internal/project` (+ the named test files) | sonnet | S | S2 | 5 |
| J-WRITELOCK | `internal/writelock` | sonnet | S | S2 | 2.2 |
| DIRCASE | `internal/build` | opus | M | S2 | 2.4 |
| N-EVAL | `internal/value`, `internal/eval`, `internal/eval/std` (+ `positional`'s rows in `internal/check`) | opus | M | S2 | 3 |
| Q-SYNTAX | `internal/syntax` | sonnet | S-M | S2 | 2.6 |
| NOHOOK | `internal/ir` | sonnet | XS | S2 | 2.5 |
| VMBENCH | `internal/testkit/cmd/benchgen` | sonnet | XS | S2 | 6 |
| Q-EDITORS | `editors/vscode`, `internal/testkit/vscodegrammar` | sonnet | XS | S2 | 2.6 |
| J-EDIT | `internal/edit` | opus | M | J-WRITELOCK | 2.2 |
| J-BUILD | `internal/build` | opus | S-M | J-WRITELOCK, DIRCASE | 2.2 |
| T-PROJECT | `internal/project` | opus | M | O2 | 4 |
| N-IR-PLANS | `internal/ir` | sonnet | XS | NOHOOK | 3 |
| Q-FORMAT | `internal/format` | sonnet | XS | Q-SYNTAX | 2.6 |
| Q-CHECK | `internal/check` | sonnet | XS | Q-SYNTAX, N-EVAL | 2.6 |
| Q-LSP | `internal/lsp` | sonnet | XS | Q-SYNTAX | 2.6 |
| Q-PROGEN | `internal/testkit/progen/grammar` | sonnet | XS | Q-SYNTAX | 2.6 |
| J-WORKSPACE | `internal/workspace` | opus | M | J-EDIT, J-BUILD, STAMP, O2 | 2.2 |
| Q-EDIT | `internal/edit` | opus | M-L | J-EDIT, Q-SYNTAX, ASSETS | 2.6 |
| ASSETS | `internal/build` (+ the granted files) | sonnet | S | J-BUILD | 2.3 |
| N-CHECK | `internal/check` (+ the three granted eval fixtures) | opus | S | Q-CHECK, N-EVAL | 3 |
| N-IR-W8031 | `internal/ir` | sonnet | S | N-IR-PLANS, N-EVAL | 3 |
| N-GO | `internal/gen/go` | sonnet | M-L | N-IR-PLANS | 3 |
| N-CPP | `internal/gen/cpp` | sonnet | S | N-IR-PLANS | 3 |
| N-TS | `internal/gen/ts` | sonnet | M | N-IR-PLANS, ASSETS | 3 |
| J-CLI | `internal/cli` | sonnet | S | J-WORKSPACE, O2 | 2.2 |
| (fixup) | `api/session.go:35` | orchestrator | XS | J-WORKSPACE | 2.2 |
| T-BUILD | `internal/build` | sonnet | S | T-PROJECT, ASSETS, N-IR-W8031 | 4 |
| T-EXAMPLES | `examples/` (+ the granted AST and scope goldens) | sonnet | S | T-BUILD, Q-SYNTAX, Q-EDITORS, N-GO, N-CPP, N-TS | 4 |
| GUIDE | `internal/cli` (`guide/*.md` and its goldens) | sonnet | S | J-CLI, T-BUILD, Q-SYNTAX, N-CHECK | 7.2 |
| CLEANUP | as found | sonnet | S | every unit | 7.2 |

- `internal/build` takes DIRCASE, J-BUILD, ASSETS, T-BUILD in that order; `internal/edit` J-EDIT then
  Q-EDIT; `internal/ir` NOHOOK, N-IR-PLANS, N-IR-W8031; `internal/check` N-EVAL (its two `positional`
  rows), Q-CHECK, then N-CHECK; `internal/project` O2 then T-PROJECT; `internal/workspace` STAMP then
  J-WORKSPACE; `internal/cli` J-CLI then GUIDE. O2 touches only test files of `internal/workspace`,
  `api` and `internal/cli`, so it may run beside STAMP; ASSETS touches only
  `internal/gen/ts/pipeline_test.go` in `gen/ts`, before N-TS.
- `internal/testkit/progen/testdata/gaps.txt` takes one line from several units: a cherry-pick
  conflict there keeps every line, in the file's order.
- The critical path is the journal: J-WRITELOCK → J-EDIT ∥ J-BUILD → J-WORKSPACE → J-CLI; start it
  first.
- **GUIDE** writes, in `internal/cli/guide/`: §4's tiers section and its two `errors.md` rows; §2.6's
  sentence (`syntax.md:127-128`: "the others are quoted (`` icon: `return` ``), which needs no import,
  or qualified (`Icon.return`)") and an `E1138` row; for N11, `W8031` and `W1912` rows and a short
  paragraph "Positions are not ids" (keys and `@stable` fields name entries; `positional(…)` writes a
  position on purpose) at the end of `internal/cli/guide/stable.md`. Every recipe passes `canon check`
  in an examples project; the guide's output goldens are regenerated and read.
- **CLEANUP**: one delegation after every unit landed, for helpers invented twice (`make audit`,
  `dup-in-repo`), then a review.
- **T-EXAMPLES**: the tiers example declares no emit, so the generator units never touch it.

**S4, acceptance and release:** §8.

**Estimate:** 31 delegations (the 30 units of the table and S2's DIAG go-dev) plus S1 and the
fixup; about 18 to 24 hours of agent time (S1 and S2 about 2 h, the units 13 to 17 h with review
rounds, acceptance and release 3 to 5 h), so 1 to 1.5 days wall, plus any rate-limit stop (resume
each agent where it stopped, as in v0.1.6). Calibration: v0.1.6 took about 15 h of agent time for
12 units, 24 h wall with a 9 h rate-limit stop.

## 8. Release

Only when every unit has landed, CLEANUP is reviewed, and the spec, DECISIONS and `meta/` are in sync
(`meta/state.md` ≤ 80 lines). Each step's command, machine, load at start and end, and result go in the
final acceptance entry of `meta/decisions/log-<date>.md`.

1. `make check` under `systemd-run --user --scope -q -p MemoryMax=6G env GOTOOLCHAIN=local`, on a clean
   `main`.
2. Long gates, one at a time, never together, no agent running:
   - `make stress` (60 s, 3G, `-race`);
   - `make fuzz-edit` (10 min, 6G; its judge floors are 2,000 execs and 200 applied edits per minute).
     Then `git status`: a crasher lands in `internal/edit/testdata/fuzz/FuzzMinimalWriteAll/`; it is a
     bug, fixed (failing test first) before the tag;
   - the bench A/B for §6: build tree A = `git archive v0.1.8` into `/var/tmp/canon-019-ab/a` plus
     VMBENCH's test files copied in, tree B = `main`; run `make bench-edit` (6G) as A, B, A, B, with
     §6 point 4's re-run rule for unequal loads; the baseline is median(A1, A2) of the refresh p95;
     apply §6 point 2 (the target) and point 4 (the regression rule); if the target is not 500 ms,
     correct IMPLEMENTATION-PLAN §7.6, DECISIONS 382 and VMBENCH's `refreshTarget` in one commit
     `docs(spec): the refresh target (DECISIONS 382)`;
   - `make bench-lsp` (6G), the same A/B on the same two trees (A, B, A, B; the same re-run rule): a
     regression of more than 10% in its p95 (median(B1, B2) > 1.10 × median(A1, A2)) blocks the tag
     until its cause is found, by re-running B with one suspect unit's commits reverted in a scratch
     worktree, in this order: STAMP, O2, J-WORKSPACE, Q-SYNTAX, N-EVAL. A regression caused by a
     robustness change (STAMP's stat fields, J-WORKSPACE's locks) is accepted (robustness first) and
     recorded in the log and the reply with its numbers; any other cause is a bug, fixed before the
     tag.
3. Cross-platform checks (CI is down): `GOOS=windows go vet ./...`, `GOOS=darwin go vet ./...`, §2.1's
   cross-build list, `go test -race` over `api`, `internal/workspace`, `internal/edit`,
   `internal/build`, `internal/writelock`, `internal/cli` at `GOMAXPROCS` 2, 4 and default.
4. **Resource dry run** (read-only on Resource and Source):
   - the candidate: `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X
     github.com/fantasim/canonlang/internal/build.CompilerVersion=0.1.9" -o /var/tmp/canon-019-dry/canon
     ./cmd/canon`;
   - the farm: `git -C ../../Resource archive HEAD | tar -x -C /var/tmp/canon-019-dry/res`; copy
     (`cp -p`) only the `@source/<file>` inputs named in `Config/*.canon` into
     `/var/tmp/canon-019-dry/src`; empty directories for the consumer roots;
   - the steps of `Resource/Tools/hooks/pre-push`, with roots placed by flags (`-root
     resource=<farm>/res -root source=<farm>/src -root monitoring=… -root admin=… -root website=…
     -root trello=…`, the empty directories): `fmt -check`; `build -max-warnings 0 -format json`; the
     clone unchanged and the Source outputs equal; a blind rebuild; `lock check`;
     `check -max-warnings 0`; `test`; `build -check -max-warnings 0`. Each runs under
     `flock -x ${XDG_RUNTIME_DIR:-/tmp}/sov-canon-heavy.lock systemd-run --user --scope -q -p
     MemoryMax=12G -p MemorySwapMax=0 nice -n 15`, once with the pinned 0.1.8 binary
     (`~/.local/bin/canon`, read only) and once with the candidate, each on a fresh farm;
   - compare exit codes, finding text, `git status --porcelain` of the farm clone (committed outputs
     that change), `diff -r` of the Source copies, wall time and RSS. Expected with the candidate: 0
     errors and 0 warnings (no `W8031`, no `W1912`: §3's scan), the same package count, and output
     diffs only where §3 changes generated code (baked id shapes). Every other diff is classified by
     rule, never by judgement: a finding whose code this ADR introduces or extends (`E3706`, `E8152`
     `dirCase`, `W8031`, `W1912`, `E1138`) is real Resource data the new rule catches: an upgrade note
     in the reply and an item in a Resource handoff, and the release goes on; anything else (another
     code, a changed message, a changed output not explained by §3) is a bug, fixed (failing test
     first) before the tag;
   - for each service whose generated Go or TS changes: `build -only-root <service>` into a
     `git archive` copy of that service, then `go vet ./...` (Go) or `tsc --noEmit` (TS) there; each
     breakage found is listed in the reply with its fix (§3's migration).
5. The final acceptance entry in the log (every result above).
6. Tag and dist, on a clean `main`: copy `dist/` to `/var/tmp/canon-dist-0.1.8/` (`make dist` starts
   with `rm -rf dist`); `make tag VERSION=v0.1.9` (annotated, local, never pushed); `make dist
   VERSION=v0.1.9` (the version is stamped by `-X …CompilerVersion`; there is no bump commit);
   `(cd dist && sha256sum -c checksums.txt)`; extract `linux_amd64` and check that `canon version`
   reads `canon 0.1.9 (<sha>)` with the tag's commit, and that its sha256 equals that of a candidate
   rebuilt at the tag on the clean tree with step 4's command (plus `GOTOOLCHAIN=local`): the binary
   embeds the commit (`internal/build/version.go:18`), so the dry-run candidate, built before step 5's
   log commit, cannot match. Step 4's results stand when `git diff --stat <dry-run commit> v0.1.9`
   touches only `meta/`; otherwise redo step 4 with the rebuilt candidate. A failure after the tag:
   `git tag -d v0.1.9` (never pushed) and redo from the failing step.
7. Never install the binary (`~/.local/bin` is outside this directory; Louis or Scripts' canon-bump
   does it) and never push `main` or the tag (Louis does).
8. The reply, committed after the tag: `meta/handoff/<date>-canon-reply-v019.md`, for the Sovereign
   sessions (Resource, Source, Scripts, website, admin, monitoring, config-studio):
   - what changed, per item of §1's table, with its DECISIONS number;
   - upgrade notes: `encoding/json`, XML and YAML now write a Go enum without `@codes` as its wire
     name, and read it back by name (values and map keys; the scan found no consumer encoding one);
     dot-prefixed `.canon` files are no sources (none is tracked in Sovereign); the
     journal is version 2, and a v0.1.8 binary refuses a dead v0.1.9 journal ("unknown version") until
     v0.1.9 recovers it: do not switch binaries on a tree an interrupted edit left; edits now write
     `` `in` `` where they wrote `Leg.in` (both stay valid); case-only `out` renames now apply (commit
     them with `git mv`); new findings that may appear: `E3706`, `E8152` `dirCase`, `W8031`, `W1912`,
     `E1138`; Windows keeps size + mtime in the refresh;
   - N11 migration (§3) and the scan's advice on order coupling outside Canon: the feed-cursor leg
     `#index` (monitoring), `PointTypeKindMembers[0]` labels (admin web UI), the telemetry X-macro
     column order, the VIP rows: give them explicit ids;
   - the recommended `tiers` for Resource (0 violations on its 356 edges), to adopt now and tighten
     after its S1 refactor:
     ```
     tiers {
       base: [header]
       ids: [ai.vocab, character.vocab, combat.vocab, content.vocab, economy.vocab, engine.vocab, events.vocab, guild.vocab, items.vocab, pet.vocab, progression.vocab, social.vocab, text.vocab, ui.vocab, world.vocab]
       data: [ai, balance, character, combat, content, dev, economy, engine, events, guild, items, model, pet, progression, social, studio, text, ui, world]
       out: [ai.client, ai.npc.client, character.client, content.clientui, items.catalog.client, items.client, items.clientui, model.client, pet.clientui, social.clientui, ui.client, ui.clientui, ui.servernames, world.client, world.clientworld, services, telemetry]
     }
     ```
     (its own target rule, `engine` below ids and `text.client` in `out`, gives 10 violating edges
     today: `engine` → 7 vocab packages, `engine.memory` → `model.skill`,
     `character/system/nmBuff.canon:8` and `guild/ranknames/rankNames.canon:4` → `text.client`);
   - the refresh target and the bench numbers (§6), any recorded miss;
   - the 5 checksum lines and the `linux_amd64` binary's sha256;
   - what stays owed (§9).
   Then `meta/handoff/README.md`, `meta/plan.md` (v0.1.9 ticked) and `meta/state.md`.
9. Louis pushes `main` and the tag and publishes with `gh release create v0.1.9 dist/*.tar.gz
   dist/*.zip dist/checksums.txt --title v0.1.9 --notes-from-tag`; Scripts' canon-bump moves the pin.

## 9. Found while planning, owed (not in v0.1.9)

Logged in `meta/state.md` "Owed, small" by the spec sync (S1); none is fixed in v0.1.9.

- **`load.dir` and Windows junctions** (read from Go's source, not run): a junction met during a walk
  is skipped silently, with no `W7115` (`internal/load/symlink.go:164-168`), so a table can lose every
  entry under it; a base or literal path through a junction is not bounded (`resolveBase` sees no
  link). The fix belongs in `project.readLink` (treat what `os.Readlink` can read, a symlink or a
  mount point, as a link) for every reader, tested on the Windows runner with `mklink /J`.
- **Single-file `load`, `load.text`, `load.csv`, `load.defines` read through any link**
  (`internal/load/single.go:46-63`; WIRE §6.1 says nothing). Unlike assets they read content; the
  load-side finding needs its own design.
- **Windows refresh:** re-read the names an OS notification or `didChangeWatchedFiles` reports
  whatever their stamp, and drop the entries a `*StaleError` names (§2.1).
- **The cache's stat table** (IMPLEMENTATION-PLAN §8.5, `(path, size, mtime, sha256)`): its future
  ADR re-hashes every read (log-2026-10-14's safe design) and must not trust size + mtime (§2.1).
- **A `@text` output may carry a `.canon` or `.json` name** (CODEGEN §2.9's `E8021` has no extension
  rule): whether a build refuses to write over an unmarked such file is unverified.

- **Enums, v0.2** (§3 points 1 and 16; agreed with Louis 2026-10-14, in the plan's v0.2): Go enums
  without `@codes` become opaque (`struct` plus `Position()`, as ids in §3 point 2); C++ deletes the
  nlohmann hooks for them, after a read-only scan of Source's use of nlohmann on Canon enums (Source
  is not compiled in the release's dry run); TypeScript stops exporting `<E>Index` (private
  `canonIndex`, as ids). Open enums are decided with them.
- **The studio's full reload** (a `ViewModel` per package plus `Check`) has no target (§6 point 3).
- **`evalAnalysis`** (`api/evaluate.go:145-158`) never calls `workspace.KeptSelection` (§6 point 5):
  performance, parked with the rest (robustness first).

## 10. Consequences

- **Breaking for consumers** (each named in the reply, §8): Go `baked` table ids become structs (a
  `const` use or an integer conversion stops compiling; JSON of an id field turns from a number into
  the key); JSON, XML and YAML of a Go enum without `@codes` (values and map keys) change from numbers
  to wire names; C++ ids refuse nlohmann; TypeScript's `<Id>Index` gives way to `<Id>Members`; dot-prefixed
  `.canon` files stop being sources; the edit journal is version 2. The scan found no site in
  Sovereign that breaks (Go 0, C++ 0, TS 0 for baked ids).
- **New language surface:** the `QWORD` token (`` `in` ``), the `tiers` project key, the `positional`
  builtin; codes `E1017`, `E1018`, `E2007`, `E1138`, `E3706`, `W1912`, `W8031` and `E8152` `dirCase`
  (331 codes).
- **Frozen contracts:** `value.Prov` gains one field in its padding (§4.3, under the review rule);
  `ir.go` gains a comment. No API change (`api/canon.go` untouched).
- **Robustness gained:** the refresh no longer trusts size + mtime on Unix-like systems; writer locks
  make recovery safe against live writers and end the transient `ErrJournal`; an asset's verdict no
  longer depends on files outside the declared roots; a case-only `out` rename applies everywhere;
  reserved-word members can be written in every file; a baked position can no longer leave as an id
  by accident; import directions are enforced; an editor's lock file is never a source.
- **Costs:** 16 bytes more per refreshed entry; one type switch per operand at the mark's carrying
  sites (bounded by §3's performance guard); an edit also locks the readers of a JSON file it writes
  (a little more waiting, never a wrong answer); an edit that meets a dead edit's journal fails once as
  stale, then applies.
- **Residual risks, accepted:** Windows keeps size + mtime in the refresh; Windows junctions stay
  unbounded under `load.dir` (§9); position detection has named blind spots (plain lists, `Range`
  counters, lists mapped from tables); Windows and macOS lock and rename behaviour is checked by vet
  and the CI test runs when CI is back; the deleted nlohmann templates are checked on g++ and clang++,
  not MSVC.
- **Unchanged on purpose:** enums keep their numbers, only Go's text encoding of those without
  `@codes` changes (§3 point 16; the rest is v0.2, §9); performance work stays parked (per-entry
  re-check, the on-disk cache, parallel stages); NFR-01's recorded Edit and Evaluate misses since
  v0.1.5 stay recorded, not chased.
- **This ADR's own risk:** it was planned from reading the code; every "Today" column is a reading,
  not a run. A unit whose failing-test-first step does not fail on today's code stops and reports (the
  reading was wrong), and the orchestrator decides under §0.
