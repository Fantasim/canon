import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseBadges, decodeBadges } from "./features/copies/expected/copies/out/ts/copies.js";
import { ToneMembers } from "./features/copies/expected/copies/out/ts/base.js";
import { parseBadges as sovParse } from "./features/copies/expected/sovcommon/copies/ts/copies.js";

const text = readFileSync(new URL("./features/copies/expected/copies/out/data/badges.json", import.meta.url), "utf8");

test("copies: the data-mode copy reads the emitted file and imports the types-mode copy beside it (CODEGEN.md §2.1, §2.8)", () => {
  const badges = parseBadges(text);
  assert.deepEqual(badges.all.map((b) => [b.id, b.tone]), [["gold", "red"], ["iron", "blue"]]);
  assert.ok(Object.isFrozen(badges.at(0)));
  assert.deepEqual([...ToneMembers], ["red", "blue"]);
  assert.equal(decodeBadges(JSON.parse(text)).find("iron").tone, "blue");
  assert.equal(sovParse(text).length, 2);
  assert.throws(() => parseBadges(text.replace("red", "green")), /\/rows\/0\/tone: unknown member/);
  assert.throws(() => parseBadges(text.replace("Badge@", "Other@")), /built from schema/);
});
