import { test } from "node:test";
import * as assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import * as a from "./nestref_data/features/nrd/nrd.js";

const text = (name) => readFileSync(new URL("./nestref_data/json/" + name, import.meta.url), "utf8");

// the data wire of a ref keyed by a ref is its chain's leaf's wire: an enum's member wire, a @json(codes) enum's code, a string, an integer, a table id (DECISIONS 456, WIRE.md §5.8, §5.9).
const chains = [
  { name: "ew", parse: a.parseHew, tables: [a.parseEw1, a.parseEw2, a.parseEw3], wire: "z2", other: "third", mapKey: "z2", key: "z2" },
  { name: "ec", parse: a.parseHec, tables: [a.parseEc1, a.parseEc2, a.parseEc3], wire: 9, other: 4, mapKey: "9", key: "alpha" },
  { name: "st", parse: a.parseHst, tables: [a.parseSt1, a.parseSt2, a.parseSt3], wire: "q", other: "p", mapKey: "q", key: "q" },
  { name: "in", parse: a.parseHin, tables: [a.parseIn1, a.parseIn2, a.parseIn3], wire: 20, other: 5, mapKey: "20", key: 20 },
  { name: "tb", parse: a.parseHtb, tables: [a.parseTb1, a.parseTb2, a.parseTb3], wire: "sword", other: "shield", mapKey: "sword", key: "sword" },
];

for (const c of chains) {
  test(`${c.name}: the file's own wire is the leaf's and it loads (DECISIONS 456)`, () => {
    const raw = JSON.parse(text("h" + c.name + ".json")).value;
    assert.equal(raw.f2, c.wire);
    assert.equal(raw.f3, c.wire);
    assert.deepEqual(raw.fl, [c.other]);
    assert.deepEqual(Object.keys(raw.fm), [c.mapKey]);
    const h = c.parse(text("h" + c.name + ".json"));
    const key = c.key;
    assert.equal(h.f2, key);
    assert.equal(h.f3, key);
    assert.deepEqual([...h.fm.keys()], [key]);
    const [one, two, three] = c.tables.map((p, i) => p(text(c.name + (i + 1) + ".json")));
    for (const t of [one, two, three]) {
      assert.equal(t.length, 2);
    }
    assert.notEqual(two.find(h.f2), null);
    assert.notEqual(three.find(h.f3), null);
    assert.notEqual(three.find(h.fl[0]), null);
    assert.equal(three.find(h.ev).shape, "count");
    assert.equal(two.find(h.ev2).shape, "place");
    assert.deepEqual({ ...h.param }, { branch: "count", value: 3 });
    assert.deepEqual({ ...h.param2 }, { branch: "place", value: "x" });
  });
}

test("a ref chain's wire is checked at the leaf: an unknown member is an error (WIRE.md §5.8)", () => {
  const bad = text("hew.json").replace('"f2": "z2"', '"f2": "second"');
  assert.notEqual(bad, text("hew.json"));
  assert.throws(() => a.parseHew(bad), /f2/);
});
