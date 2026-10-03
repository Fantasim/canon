import { test } from "node:test";
import * as assert from "node:assert/strict";
import * as tb from "./teamboard/expected/web/teamboard/generated.js";
import { RoleIndex, RoleMembers } from "./teamboard/expected/web/roles.generated.js";
import { ToneMembers, ToneNames, IconIndex, IconMembers } from "./teamboard/expected/web/ui.generated.js";

test("teamboard: tables find by id, keep source order and retired rows, and are frozen", () => {
  assert.equal(tb.statuses.length, 6);
  assert.deepEqual(tb.statuses.all.map((s) => s.id), ["open", "taken", "fixed", "verified", "wont_do", "duplicate"]);
  assert.equal(tb.statuses.find("taken").tone, "info");
  assert.equal(tb.statuses.find("nope"), null);
  assert.equal(tb.statuses.at(0), tb.statuses.find("open"));
  assert.deepEqual([...tb.statuses.find("open").next], ["taken", "wont_do", "duplicate"]);
  assert.ok(Object.isFrozen(tb.statuses) && Object.isFrozen(tb.statuses.at(0)) && Object.isFrozen(tb.statuses.at(0).next));
  assert.equal(tb.StatusIdIndex.duplicate, 5);
  assert.equal(tb.areas.length, 12);
  assert.equal(tb.areas.find("game").group, "in_game");
  assert.equal(tb.deck.maxHidden, 2);
  assert.deepEqual([...tb.deck.layouts], ["4:1", "2:2"]);
  assert.equal(tb.initialStatus, "open");
  assert.equal(tb.defaultSeverity, "normal");
  assert.equal(tb.assigneeMinRole, "maintainer");
});

test("teamboard: lookups by ordinal, over an imported enum and own ids (CODEGEN.md §5.10)", () => {
  assert.equal(tb.canTransition("open", "taken"), true);
  assert.equal(tb.canTransition("open", "fixed"), false);
  assert.equal(tb.canTransition("verified", "open"), true);
  assert.equal(tb.columnOf("verified"), "done");
  assert.equal(tb.columnOf("open"), "unclaimed");
  assert.deepEqual([...tb.areasVisibleTo("member")], []);
  assert.ok(tb.areasVisibleTo("admin").length === tb.areas.length);
  for (const role of RoleMembers) {
    for (const id of tb.areasVisibleTo(role)) assert.ok(RoleIndex[role] >= RoleIndex[tb.areas.find(id).minRole]);
  }
  assert.deepEqual(tb.groupedAreasVisibleTo("gm_junior").map((s) => s.group), ["in_game", "out_game", "internal"]);
  assert.ok(Object.isFrozen(tb.areasVisibleTo("admin")));
});

test("ui and roles: enum tables (CODEGEN.md §5.2)", () => {
  assert.equal(ToneMembers.length, 12);
  assert.equal(ToneNames["series-1"], "series_1");
  assert.equal(IconIndex.archive, 1);
  assert.ok(IconMembers.includes("alert-triangle"));
  assert.ok(RoleIndex.gm_junior < RoleIndex.maintainer);
  assert.ok(Object.isFrozen(RoleMembers) && Object.isFrozen(ToneNames));
  for (const t of tb.statuses.all) assert.ok(ToneMembers.includes(t.tone));
});
