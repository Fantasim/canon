import { test } from "node:test";
import * as assert from "node:assert/strict";
import { offers, OfferIdIndex } from "./emptytable/features/a/a.js";

test("an empty table (CODEGEN.md §5.3)", () => {
  assert.equal(offers.length, 0);
  assert.deepEqual({ ...OfferIdIndex }, {});
});
