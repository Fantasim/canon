import { test } from "node:test";
import * as assert from "node:assert/strict";
import { extra, items } from "./rowkeyed_baked/features/a/a.js";

test("a keyed list's element of a row record has no id; the table's row has its own (DECISIONS 279(c))", () => {
  assert.deepEqual({ ...items.find("i1") }, { sku: "i1" });
  assert.equal("id" in items.find("i1"), false);
  assert.deepEqual({ ...extra.find("one") }, { id: "one", retired: false, sku: "one" });
});
