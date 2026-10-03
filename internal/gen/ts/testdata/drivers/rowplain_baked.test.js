import { test } from "node:test";
import * as assert from "node:assert/strict";
import { ps, single, qs } from "./rowplain_baked/features/a/a.js";

test("a row record held as a plain value has no id; table entries have theirs (CODEGEN.md §5.4)", () => {
  assert.deepEqual({ ...single }, { x: 5 });
  assert.equal("id" in single, false);
  assert.deepEqual({ ...ps.find("p1") }, { id: "p1", retired: false, x: 1 });
  assert.equal(ps.find("p2").retired, true);
  const q = qs.find("a");
  assert.deepEqual({ ...q.inner }, { x: 3 });
  assert.deepEqual(q.slots.map((s) => ({ ...s })), [{ id: "s1", retired: false, x: 4 }]);
});
