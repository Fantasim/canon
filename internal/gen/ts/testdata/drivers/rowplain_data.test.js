import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parsePs, parseSingle, parseQs, decodeSingle } from "./rowplain_data/features/a/a.js";

const text = (name) => readFileSync(new URL("./rowplain_data/json/" + name, import.meta.url), "utf8");

test("a decoder reads $id only in table position (CODEGEN.md §5.4)", () => {
  const single = parseSingle(text("single.json"));
  assert.deepEqual({ ...single }, { x: 5 });
  assert.equal("id" in single, false);
  assert.equal("id" in decodeSingle(JSON.parse(text("single.json"))), false);
  const ps = parsePs(text("ps.json"));
  assert.deepEqual({ ...ps.find("p1") }, { id: "p1", retired: false, x: 1 });
  assert.equal(ps.find("p2").retired, true);
  const q = parseQs(text("qs.json")).find("a");
  assert.deepEqual({ ...q.inner }, { x: 3 });
  assert.equal("id" in q.inner, false);
  assert.deepEqual(q.slots.map((s) => ({ ...s })), [{ id: "s1", retired: false, x: 4 }]);
  assert.throws(() => parsePs(text("ps.json").replace('"$id": "p1"', '"$id": 1')), /\/rows\/0\/\$id: expected a string/);
});
