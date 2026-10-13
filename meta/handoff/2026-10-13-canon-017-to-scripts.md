# To Scripts: move the canon pin to 0.1.7 (2026-10-13)

From the Canon compiler session. canon **v0.1.7** is published
(https://github.com/Fantasim/canon/releases/tag/v0.1.7) and installed at `~/.local/bin/canon`
from the release archive. Skip 0.1.6: bump once, straight to 0.1.7.

**Do:** run your `canon-bump` skill with `V=0.1.7`. Both pins still say 0.1.5
(`lib/canon_gate.sh:144` and Resource `Tools/hooks/pre-push:49`), so the gate and Resource's
pre-push hook refuse the installed binary until the bump lands.

- Step 1's expected hash `H` = `e50b09d61972b8f70507781d83fae3a0b62783b77aad05f43cc77aa74f8b7b8b`. That
  is the sha256 of `~/.local/bin/canon`, equal to the `canon` inside `canon_0.1.7_linux_amd64.tar.gz`
  (checked after install).
- Step 3 still needs its own two runs. For reference, one run each on a copy of Resource HEAD
  `43e46174` under a 6 GB cap:

  | Step | 0.1.5 | 0.1.6 |
  |---|---:|---:|
  | `check` | 17.1 s, 2.56 GB | 18.5 s, 2.66 GB |
  | `build --check` | 22.1 s, 2.98 GB | 22.3 s, 2.95 GB |

  0.1.7 changes only generated TypeScript, so expect the same figures.
- Step 4/5: the Resource section needs only the pins, the README line and the memory comment.
  The other Resource work is in [the Resource handoff](2026-10-13-canon-017-to-resource.md),
  separate from the bump.

What changed in canon: [v0.1.6 reply](2026-10-13-canon-reply-v016.md) and the v0.1.7 release
notes. In short: the language server releases closed projects (the leak in
`meta/handoffs/2026-10-12-canon-lsp-memory-leak.md`), and large generated TS tables compile.
Please re-measure your `canon lsp` gate after the bump.
