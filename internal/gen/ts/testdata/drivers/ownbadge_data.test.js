import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseHolders, parseMine, parseTheirs } from "./ownbadge_data/features/a/a.js";

const text = (name) => readFileSync(new URL("./ownbadge_data/json/" + name, import.meta.url), "utf8");

test("a package's own Badge and another package's Badge each read by their own reader (log-2026-10-06 \"TS reader names\")", () => {
  const h = parseHolders(text("holders.json")).find("h");
  assert.deepEqual({ ...h.mine }, { name: "m", rank: 2 });
  assert.deepEqual({ ...h.theirs }, { label: "t", weight: 1 });
  assert.deepEqual({ ...parseMine(text("mine.json")).find("gold") }, { id: "gold", retired: false, name: "Gold", rank: 0 });
  assert.deepEqual({ ...parseTheirs(text("theirs.json")).find("Silver") }, { label: "Silver", weight: 0.5 });
  assert.throws(() => parseHolders(text("holders.json").replace('"label": "t"', '"label": 1')), /\/rows\/0\/theirs\/label: expected a string/);
});
