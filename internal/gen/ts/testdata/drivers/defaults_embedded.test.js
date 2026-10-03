import { test } from "node:test";
import * as assert from "node:assert/strict";
import { os } from "./defaults_embedded/features/a/a.js";

test("defaults carry their stored results (DECISIONS 278)", () => {
  assert.deepEqual({ ...os.find("a").p }, { x: 1, double: 2 });
  assert.deepEqual({ ...os.find("a").sh }, { kind: "dot", area: 0 });
  assert.deepEqual({ ...os.find("b").p }, { x: 4, double: 8 });
  assert.deepEqual({ ...os.find("b").sh }, { kind: "circle", r: 2 });
});
