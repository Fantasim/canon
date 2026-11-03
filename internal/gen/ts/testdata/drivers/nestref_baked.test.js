import { test } from "node:test";
import * as assert from "node:assert/strict";
import * as a from "./nestref_baked/features/nrb/nrb.js";

// every key a holder names is a key of its chain's lists, written as the leaf is: DECISIONS 456, WIRE.md §5.8, §5.9.
const chains = [
  { name: "ew", holder: a.hew, lists: [a.ew1, a.ew2, a.ew3], key: "z2", other: "third" },
  { name: "ec", holder: a.hec, lists: [a.ec1, a.ec2, a.ec3], key: "alpha", other: "zulu" },
  { name: "st", holder: a.hst, lists: [a.st1, a.st2, a.st3], key: "q", other: "p" },
  { name: "in", holder: a.hin, lists: [a.in1, a.in2, a.in3], key: 20, other: 5 },
  { name: "tb", holder: a.htb, lists: [a.tb1, a.tb2, a.tb3], key: "sword", other: "shield" },
];

for (const c of chains) {
  test(`${c.name}: a ref two and three deep is its leaf's key in a field, a list and a map key (DECISIONS 456)`, () => {
    const [, two, three] = c.lists;
    assert.equal(c.holder.f2, c.key);
    assert.equal(c.holder.f3, c.key);
    assert.deepEqual([...c.holder.fl], [c.other]);
    assert.deepEqual([...c.holder.fm], [[c.key, 1]]);
    assert.notEqual(two.find(c.holder.f2), null);
    assert.notEqual(three.find(c.holder.f3), null);
    assert.notEqual(three.find(c.holder.fl[0]), null);
    assert.notEqual(three.find([...c.holder.fm.keys()][0]), null);
    assert.deepEqual(two.all.map((r) => r.k), [c.key, c.other]);
    assert.deepEqual(three.all.map((r) => r.k), [c.key, c.other]);
  });

  test(`${c.name}: a dependent branch table and its values decide through the chain (DECISIONS 456)`, () => {
    const three = c.lists[2];
    assert.deepEqual({ ...c.holder.param }, { branch: "count", value: 3 });
    assert.deepEqual({ ...c.holder.param2 }, { branch: "place", value: "x" });
    assert.equal(three.find(c.holder.ev).shape, "count");
    assert.equal(c.lists[1].find(c.holder.ev2).shape, "place");
  });
}

test("a literal union arm is the leaf's wire (DECISIONS 387, 456)", () => {
  assert.equal(a.hew.fu, "z2");
  assert.deepEqual([...a.hew.fus], ["none", "third"]);
  assert.equal(a.hst.fu, "q");
  assert.deepEqual([...a.hst.fus], ["none", "p"]);
  assert.equal(a.htb.fu, "sword");
});
