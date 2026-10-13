import { test } from "node:test";
import * as assert from "node:assert/strict";
import { decodeUse, parseItemsFile } from "./open_foreign/features/a/a.js";

test("an importer reads any string for another package's opened enum (CODEGEN.md §2.8, §5.2, DECISIONS 362)", () => {
  const u = decodeUse({ dst: "later", place: "all", counts: { hp: 1, newer: 2 } });
  assert.equal(u.dst, "later");
  assert.deepEqual([...u.counts], [["hp", 1], ["newer", 2]]);
  assert.throws(() => decodeUse({ dst: 3 }), new Error("Use: $.dst: expected a string"));
});

test("a @text result that reaches another package through its records is decoded (DECISIONS 363)", () => {
  assert.deepEqual(parseItemsFile('[{"dst": "x"}, {"dst": "hp"}]').map((i) => i.dst), ["x", "hp"]);
  assert.throws(() => parseItemsFile('[{"dst": 1}]'), new Error("ItemsFile: $[0].dst: expected a string"));
});
