import { test } from "node:test";
import * as assert from "node:assert/strict";
import { rs } from "./bigkeys_embedded/features/a/a.js";

test("refs and map keys into a bigint key field are bigints (DECISIONS 278)", () => {
  const a = rs.find(9007199254740993n);
  assert.equal(a.n, 9007199254740993n);
  assert.equal(a.next, 9007199254740993n);
  assert.equal(rs.find(a.next), a);
  assert.equal(rs.find(rs.find(1n).next), a);
  assert.deepEqual([...a.m], [[9007199254740995n, 1n]]);
  assert.deepEqual([...a.l], [9007199254740997n]);
  assert.equal(a.mx, 9007199254740999n);
  assert.deepEqual([...a.w], [[1n, 2]]);
  assert.equal(rs.find(1n).mx, null);
});
