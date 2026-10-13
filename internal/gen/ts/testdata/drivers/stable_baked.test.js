import { test } from "node:test";
import * as assert from "node:assert/strict";
import * as a from "./stable_baked/features/a/a.js";

test("a @stable field gets findBy<Field> on its container, retired rows included (CODEGEN.md §5.9, DECISIONS 369)", () => {
  assert.equal(a.statuses.findByCode(2).id, "closed");
  assert.equal(a.statuses.findByTag("OPEN").code, 1);
  assert.equal(a.statuses.findByCode(7).retired, true);
  assert.equal(a.statuses.findByTag("STALE").id, "stale");
  assert.equal(a.statuses.findByCode(3), null);
  assert.equal(a.statuses.findByTag("open"), null);
  assert.equal(a.statuses.findByCode(2), a.statuses.find("closed"));
  assert.ok(Object.isFrozen(a.statuses));
});
