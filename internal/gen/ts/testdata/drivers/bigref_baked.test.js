import { test } from "node:test";
import * as assert from "node:assert/strict";
import { h, ks } from "./bigref_baked/features/a/a.js";

test("a ref keeps its target key's form beside @ts(bigint) positions (DECISIONS 279(a))", () => {
  assert.deepEqual([...h.m], [[1, 9007199254740995n]]);
  assert.deepEqual([...h.byBig], [[9007199254740997n, 2]]);
  assert.equal(h.b, 9007199254740993n);
  assert.deepEqual([...h.bk], [[9007199254740993n, 1]]);
  assert.equal(ks.find([...h.m.keys()][0]).n, 1);
});
