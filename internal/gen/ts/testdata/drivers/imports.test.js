import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { decodeAwards, parseAwards } from "./imports/features/user/user.js";
import { badgeFor, visible } from "./imports/features/lookups/lookups.js";
import { badges, RoleIndex } from "./imports/features/shared/base.js";

test("an imported enum, table id and types-mode record are decoded across modules (CODEGEN.md §2.8, §5.8)", () => {
  const awards = decodeAwards(JSON.parse(readFileSync(new URL("./imports/json/awards.json", import.meta.url), "utf8")));
  const a = awards.find("a");
  assert.equal(a.badge, "gold");
  assert.equal(a.tone, "red");
  assert.equal(a.weights.get("red"), 1);
  assert.deepEqual({ ...a.pin }, { place: "here", count: 1, weight: 2.5 });
  assert.ok(Object.isFrozen(a.pin));
});

test("another module's decoder reads parse's number tokens and key order (DECISIONS 278)", () => {
  const text = readFileSync(new URL("./imports/json/awards.json", import.meta.url), "utf8");
  assert.equal(parseAwards(text).find("a").pin.weight, 2.5);
  assert.equal(parseAwards(text.replace('"weight": 2.5', '"weight": 1e1')).find("a").pin.weight, 10);
  assert.throws(() => parseAwards(text.replace('"count": 1', '"count": 1.0')), /\/rows\/0\/pin\/count: expected an integer/);
});

test("a foreign decoder's failure is reported at one JSON path, the field's joined with the decoder's (CODEGEN.md §5.13)", () => {
  const doc = JSON.parse(readFileSync(new URL("./imports/json/awards.json", import.meta.url), "utf8"));
  doc.rows[0].pin = { place: 3 };
  assert.throws(() => decodeAwards(doc), /^Error: \/rows\/0\/pin\/place: expected a string$/);
  doc.rows[0].pin = 3;
  assert.throws(() => decodeAwards(doc), /^Error: \/rows\/0\/pin: expected an object$/);
  doc.rows[0].pin = { place: "x" };
  doc.rows[0].tone = "green";
  assert.throws(() => decodeAwards(doc), /\/rows\/0\/tone: unknown member/);
});

test("a lookup over an imported enum indexes through its Index (CODEGEN.md §5.10)", () => {
  assert.equal(RoleIndex.admin, 1);
  assert.equal(visible("member"), false);
  assert.equal(visible("admin"), true);
  assert.equal(badgeFor("member"), "iron");
  assert.equal(badgeFor("admin"), "gold");
  assert.equal(badges.find(badgeFor("admin")).label, "Gold");
});
