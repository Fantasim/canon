import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseHs, parseBadges, parseBest, decodeBadges } from "./foreign_data/features/a/a.js";

const text = (name) => readFileSync(new URL("./foreign_data/json/" + name, import.meta.url), "utf8");

test("another package's record decodes through this file's own reader (DECISIONS 278, 323)", () => {
  assert.deepEqual({ ...parseHs(text("hs.json")).find("h").b }, { label: "x", weight: 1 });
  const badges = parseBadges(text("badges.json"));
  assert.equal(badges.find("gold").weight, 2.5);
  assert.equal(decodeBadges(JSON.parse(text("badges.json"))).find("gold").weight, 2.5);
  assert.deepEqual({ ...parseBest(text("best.json")) }, { label: "best", weight: 1 });
  assert.throws(() => parseBadges(text("badges.json").replace('"label": "gold"', '"label": 3')), /\/rows\/0\/label: expected a string/);
});
