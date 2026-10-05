import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parseZones, parseBands, parseMoods, parseMedals, parseStart, parsePerks, parseShelf, decodeBands, decodeZones } from "./shared_data/features/world/world.js";

const text = (name) => readFileSync(new URL("./shared_data/json/" + name, import.meta.url), "utf8");
const json = (name) => JSON.parse(text(name));

test("rows of another package's record read by this file's own reader, with this package's ids (CODEGEN.md §2.8, §5.9, DECISIONS 323)", () => {
  const bands = parseBands(text("bands.json"));
  const low = bands.find("low");
  assert.deepEqual([low.id, low.retired, low.status, low.width], ["low", false, "open", 4]);
  assert.deepEqual([...low.weights], [["open", 2], ["shut", 1]]);
  assert.deepEqual({ ...low.statusFor }, { gold: "open", mood: "shut" });
  assert.equal(bands.find("old").retired, true);
  assert.ok(Object.isFrozen(bands.find("low")));
  const medals = parseMedals(text("medals.json"));
  assert.deepEqual(medals.all.map((m) => [m.id, m.retired, m.label]), [["bronze", false, "Bronze"], ["tin", true, "Tin"]]);
  const moods = parseMoods(text("moods.json"));
  assert.deepEqual(moods.all.map((m) => [m.id, m.retired, m.label]), [["calm", false, "Calm"], ["grim", true, "Grim"]]);
  const start = parseStart(text("start.json"));
  assert.equal("id" in start, false);
  assert.equal(start.width, 1);
});

test("a table field, a variant and dependent values of another package's types (CODEGEN.md §2.8, §5.6)", () => {
  const z = parseZones(text("zones.json")).find("forest");
  assert.deepEqual(z.markers.map((m) => [m.id, m.retired, m.code]), [["gate", false, "g"], ["well", true, "w"]]);
  assert.deepEqual({ ...z.reward }, { kind: "coins", amount: 3 });
  assert.deepEqual({ ...z.effect.bonus }, { branch: "gold", value: 2 });
  assert.deepEqual({ ...parsePerks(text("perks.json")).find("rich").bonus }, { branch: "gold", value: 7 });
  const shelf = parseShelf(text("shelf.json"));
  assert.deepEqual(shelf.slots.map((m) => [m.id, m.retired, m.code]), [["top", false, "t"], ["low", true, "l"]]);
});

test("a failure inside another package's class names this file's JSON path (CODEGEN.md §2.8)", () => {
  const bands = json("bands.json");
  bands.rows[0].min = "one";
  assert.throws(() => decodeBands(bands), /^Error: \/rows\/0\/min: expected an integer$/);
  const noID = json("bands.json");
  delete noID.rows[1].$id;
  assert.throws(() => decodeBands(noID), /^Error: \/rows\/1\/\$id: expected a string$/);
  const zones = json("zones.json");
  zones.rows[0].markers.gate.$retired = 1;
  assert.throws(() => decodeZones(zones), /^Error: \/rows\/0\/markers\/gate\/\$retired: expected a boolean$/);
  const entry = json("zones.json");
  entry.rows[0].levels.status = "nope";
  assert.throws(() => decodeZones(entry), /^Error: \/rows\/0\/levels\/status: no entry nope$/);
  const band = json("bands.json");
  band.rows[1].status = "gone";
  assert.throws(() => decodeBands(band), /^Error: \/rows\/1\/status: no entry gone$/);
  const key = json("bands.json");
  key.rows[0].weights = { open: 2, nope: 1 };
  assert.throws(() => decodeBands(key), /^Error: \/rows\/0\/weights\/nope: no entry nope$/);
  const cell = json("bands.json");
  cell.rows[0].$statusFor.gold = "nope";
  assert.throws(() => decodeBands(cell), /^Error: \/rows\/0\/\$statusFor\/gold: no entry nope$/);
  const bonus = json("zones.json");
  bonus.rows[0].effect.bonus = "loud";
  assert.throws(() => decodeZones(bonus), /^Error: \/rows\/0\/effect\/bonus: expected an integer$/);
});
