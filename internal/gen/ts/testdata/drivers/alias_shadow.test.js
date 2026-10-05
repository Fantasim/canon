import { test } from "node:test";
import * as assert from "node:assert/strict";
import { gate, gatePoints, gateTag, gateBonus, above } from "./alias_shadow/features/a/a.js";

test("a parameter or local named like an import alias is escaped, so roles.* still reaches the import (CODEGEN.md §3.4)", () => {
  assert.equal(gatePoints(gate, 2), 10);
  assert.equal(gatePoints({ min: "member", level: 1 }, 2), 3);
  assert.equal(gateTag(gate, 4), "admin:4");
  assert.equal(gateBonus(gate, 1), 4);
  assert.equal(gateBonus({ min: "guest", level: 1 }, 1), 0);
  assert.deepEqual(["guest", "member", "admin"].map(above), [false, false, true]);
});
