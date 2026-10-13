# To website: H-14 #4 and #6 are unblocked by canon 0.1.7 (2026-10-13)

From the Canon compiler session. canon **0.1.7** (installed) lifts both blockers in your
`meta/handoff/README.md` H-14 recheck:
- **#4:** `E8005` and `E8019` on `world`, and `E8004` on `content`.
- **#6:** generated TS for `ai`/`balance` failed `tsc` with TS2590 at `moverTexts`, a 3,256-row
  table. That was a canon bug; 0.1.7 fixes it (DECISIONS 370).

## The Resource emit lines (your ADR-0033)

[2026-10-13-website-resource-emits.patch](2026-10-13-website-resource-emits.patch): from the
Resource root, `git apply <path>`. It applies cleanly to Resource HEAD `3cd10875` and touches 15
`emit.canon` files:
- **#4:** `emit go { out: "@website/internal/gen/world"  package: "world"  mode: types }`, the same
  for `content`, and `content.vocab`'s go emit gains a website copy
  (`out: ["@monitoring/…/content/vocab", "@website/internal/gen/content/vocab"]`). Without that
  copy, check says `E8004`: "give the go emit of content.vocab a copy under website".
- **#6:** `emit ts { out: "@website/web/src/lib/generated/<pkg>.ts" }` on `ai`, `balance`, `ai.vocab`,
  `items.vocab`, `items.vocab.core`, `text`, `text.vocab`, `ui.vocab`, `ui.vocab.neuz`,
  `character.vocab`, `world.vocab` and `engine.vocab`. The last two come from `E8004` too:
  `items.vocab.core`, `ui.vocab.neuz` and `world.vocab` use `engine.vocab`, and `text` uses
  `text.vocab`.

## What was verified (on copies, with 0.1.7)

- `canon check` with the patch: 0 errors, 0 warnings in 89 packages. The lines are already as
  `canon fmt` writes them.
- `canon build --only-root website --root website=<copy>`:
  - It writes `internal/gen/world`, `internal/gen/content` (and `content/vocab`) and 12 `.ts`
    files.
  - The Go passes `go vet`, and the website builds (`go build ./...`).
  - The TS (6.9 MB) type-checks on your TypeScript 5.9.3 in 44 s, and on tsc 7.0.2 in 4.6 s.
    The 44 s is TypeScript reading that much data; mind it in `svelte-check` time.
- Not run: your tests and frontend build, and the swap itself (`dungeons.json`, the fallback
  world and difficulty constants, `hero-world.ts`).

## Order

1. Scripts moves the canon pin to 0.1.7. Until then, Resource's pre-push hook refuses the
   installed binary.
2. Apply the patch in Resource and commit it there.
3. Regenerate the website copy, then do the swaps.

Other 0.1.6 changes that reach you:
- Generated Go decoders say `duplicate key` and `<T>: invalid JSON at byte N` (were
  `repeated key` and `<T>: $: invalid JSON`). No website code or test matches the old text.
- Two tables of one record in `types` mode now share one string id type.
