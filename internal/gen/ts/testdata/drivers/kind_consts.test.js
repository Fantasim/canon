import { test } from "node:test";
import * as assert from "node:assert/strict";
import { K, KS, BK } from "./kind_consts/features/a/a.js";

test("constants of a variant's kind (CODEGEN.md §4.4, §5.1)", () => {
  assert.equal(K, "dot");
  assert.equal(BK, "gold");
  assert.deepEqual([...KS], ["dot", "box"]);
  assert.equal(Object.isFrozen(KS), true);
});
