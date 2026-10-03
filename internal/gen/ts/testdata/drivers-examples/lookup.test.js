import { test } from "node:test";
import * as assert from "node:assert/strict";
import { canUpgrade, upgradeCost, GradeMembers, TierMembers, GradeIndex, TierIndex } from "./features/lookup/expected/lookup/out/lookup.generated.js";

test("lookup: every cell of the dense table is the evaluator's answer (CODEGEN.md §5.10)", () => {
  for (const g of GradeMembers) {
    for (const t of TierMembers) {
      const want = g === "normal" || (g === "unique" && TierIndex[t] >= TierIndex.pro) || (g === "ultimate" && TierIndex[t] >= TierIndex.master);
      assert.equal(canUpgrade(g, t), want, g + " " + t);
    }
  }
  assert.deepEqual(GradeMembers.map((g) => upgradeCost(g)), [10000, 500000, 20000000]);
  assert.equal(GradeIndex.ultimate, 2);
});
