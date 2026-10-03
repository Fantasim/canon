import { test } from "node:test";
import * as assert from "node:assert/strict";
import { decodeP, decodeQ, parseQ } from "./rowplain_types/features/a/a.js";

test("types mode: a plain P has no id; a nested table's rows take their keys (CODEGEN.md §5.4, §5.13)", () => {
  assert.equal("id" in decodeP({ x: 1 }), false);
  const q = parseQ('{"k": "a", "inner": {"x": 3}, "slots": {"s1": {"x": 4}}}');
  assert.deepEqual({ ...q.inner }, { x: 3 });
  assert.deepEqual(q.slots.map((s) => ({ ...s })), [{ id: "s1", retired: false, x: 4 }]);
  assert.deepEqual([...decodeQ({ k: "b", inner: { x: 1 } }).slots], []);
});
