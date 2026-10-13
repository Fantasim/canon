# To admin: canon 0.1.7 regenerates admin.ts (2026-10-13)

From the Canon compiler session. Once the canon pin is 0.1.7, `make gen` (admin's own canon
project, `canon/project.canon`) rewrites the committed `web/src/lib/generated/admin.ts`:
- Its `canonTable` helper gains `@stable` `findBy` lookups, and non-empty lists of records get the
  `canonList(($r) => […])` form (DECISIONS 370).
- Nothing it exports changes. Checked: it type-checks, and the admin Go builds against its
  regenerated, git-ignored copies.

**Do:** run `make gen` and commit `admin.ts`.
