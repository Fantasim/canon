# ADR-0010 — The edit journal: crash recovery and its threat model

Date: 2026-09-29. Status: accepted (own-process liveness refined with U5b).

## Context

API.md §10.3 (N9–N12) and O5 ask for a journal written before an edit renames anything and rolled
back at `Open`. The spec leaves open what the journal records, how much it is trusted, how it
meets symbolic links, other processes and files changed after a crash, and how its writes are made
durable through an FS interface that has no Sync. `Recover` runs on every `Open`, so a journal
shipped inside a cloned repository is input an attacker can write; a hostile `project.canon` can
also declare roots anywhere (SPEC §4).

## Decision

- **Contents.** `.canon/journal/<revision hex>.json` (no `:` for Windows): for each target its
  path (relative inside the project, absolute under a declared root outside it), whether it
  existed, its old bytes and permission bits, and the SHA-256 of its new content (or absence);
  the directories the edit creates and removes (with modes); the writer's host and pid.
- **One edit at a time.** Any journal present blocks every `Commit` until it is recovered.
- **All or nothing from a known state.** Rollback and `Recover` first check every file: already
  old — left; exactly the journal's new content — restored; anything else — the journal is kept,
  `ErrJournal` names the files, nothing is written (`Open` fails; the user resolves). Rollback
  restores only what this commit changed. A created directory is cleared only if it holds nothing
  but the journal's new files, their stages and their temp leftovers.
- **Untrusted input.** Before any write: paths confined to the project or a current declared root
  (display and absolute forms must round-trip), no `.`-prefixed segment, no symbolic link on any
  component (dangling included; `removeAll` removes a link, never descends it), file extensions
  `.canon`, `.json`, `.lock` only, file modes read/write bits only. A journal from another host is
  kept, never applied. A journal whose writer is alive on this host is left alone (liveness
  injected by the caller); for this very process, only while one of its commits runs in that
  directory (an in-process registry), so `Close` + `Open` recovers a journal a failed rollback left.
- **Durability.** An optional `SyncDir(dir) error` FS capability (the OS FS has it): the journal's
  directory before the first rename; every touched directory before the journal is removed. The OS
  `WriteFile` is temp file + fsync + rename + directory sync, its temp names hidden and containing
  the target's base name so `Recover` can clear them.
- **Editability follows.** A value from a JSON source whose extension is not `.json`, or an N2 key
  starting with `.`, is not editable (the journal could not recover the edit). `Commit` refuses a
  change through a symbolic link below the project or a root, so no commit is made whose rollback
  would refuse.

## Consequences

Overwriting or deleting an existing file through a forged journal needs its exact bytes. Residuals,
accepted: a forged journal naming this host can create new `.canon`/`.json`/`.lock` files wherever a
declared root reaches; a reused pid keeps a dead writer's journal "running"; the cross-process idle
check is not atomic; OS droppings (`.DS_Store`) in a created directory keep a journal until the user
deletes them; symlinked package directories cannot be edited through the API.
