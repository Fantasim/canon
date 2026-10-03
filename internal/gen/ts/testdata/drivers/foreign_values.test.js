import { test } from "node:test";
import * as assert from "node:assert/strict";
import { pts, one } from "./foreign_values/features/a/a.js";

test("baked values of another package's record (CODEGEN.md §2.8)", () => {
  assert.deepEqual(pts.all.map((p) => p.x), [1, 2]);
  assert.equal(pts.find(2).x, 2);
  assert.deepEqual({ ...one }, { x: 3 });
});
