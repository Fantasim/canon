import { test } from "node:test";
import * as assert from "node:assert/strict";
import { visible, total, pair } from "./escape_params/features/a/a.js";

test("escaped parameters take a $ suffix and meet no neighbour (CODEGEN.md §3.4)", () => {
  assert.equal(visible("member", true), true);
  assert.equal(visible("member", false), false);
  assert.equal(visible("guest", true), false);
  assert.equal(total(1, 2), 6);
  assert.equal(total(0, 5), 10);
  assert.deepEqual({ ...pair }, { default: 1, default_: 2 });
});
