import { test } from "node:test";
import * as assert from "node:assert/strict";
import * as a from "./emptytable/features/a/a.js";

const { offers } = a;

test("an empty table (CODEGEN.md §5.3)", () => {
  assert.equal(offers.length, 0);
  assert.deepEqual([...a.OfferIdMembers], []);
  assert.equal("OfferIdIndex" in a, false);
});
