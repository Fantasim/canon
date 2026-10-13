import { test } from "node:test";
import * as assert from "node:assert/strict";
import * as a from "./stable_shapes/features/a/a.js";

test("an override renames the property, not the method (CODEGEN.md §5.9, DECISIONS 369)", () => {
  assert.equal(a.items.findByCode(2).name, "b");
  assert.equal(a.items.findByCode(2).theCode, 2);
  assert.equal(a.items.findByCode(9), null);
});

test("a keyed list of the record gets the lookups too (DECISIONS 369)", () => {
  assert.equal(a.listed.findByCode(3).name, "x");
  assert.equal(a.listed.findBySerial(7n).name, "x");
  assert.equal(a.listed.findBySerial(8n), null);
  assert.equal(a.listed.find("x"), a.listed.findByCode(3));
});

test("a bigint @stable field is found by a bigint (DECISIONS 369)", () => {
  assert.equal(a.items.findBySerial(9007199254740993n).id, "a");
  assert.equal(a.items.findBySerial(5n).id, "b");
  assert.equal(a.items.findBySerial(6n), null);
});
