package ir_test

import "github.com/fantasim/canonlang/internal/diag"

// hintRow is one expected E8005 finding of a program, with the way out its message names: a text swap (the Canon rename, the @cpp(name:) on the declaration, the entry renamed), none for a generator-fixed name, whose way out is a use of a type of the defining package.
type hintRow struct {
	foreignHint
	swap []string
}

// hintProgram is a package set the finding names are checked on: every package it holds besides `a`, which writes the rows' names as defines.
type hintProgram struct {
	files map[string]string
	rows  []hintRow
}

func defineRow(name, item string, swap ...string) hintRow {
	return hintRow{foreignHint{diag.E8005.AtForeignDefine, name, item}, swap}
}

func derivedRow(name, item string, swap ...string) hintRow {
	return hintRow{foreignHint{diag.E8005.AtForeignDerived, name, item}, swap}
}

func idRow(name, item string, swap ...string) hintRow {
	return hintRow{foreignHint{diag.E8005.AtForeignId, name, item}, swap}
}

func fixedRow(name, item string) hintRow {
	return hintRow{foreignHint{diag.E8005.AtForeignFixed, name, item}, nil}
}

const (
	hintDataSrc = `package p

/// A tier.
record Tier {
  /// Its title.
  title: String
  /// Its rank.
  rank: Int @stable
}

/// The tiers.
let tiers: stable table Tier = {
  x { title: "a", rank: 1 }
}

emit cpp { out: "@features/p", namespace: "p", mode: data }
emit json { out: "out/" }
`
	hintBakedSrc = `package p

/// A tier.
record Tier {
  /// Its rank.
  rank: Int
}

/// The tiers.
let tiers: table Tier = {
  x { rank: 1 }
  zed { rank: 2 }
}

/// A unit.
record Unit {
  /// Its tier.
  tier: ref tiers
}

/// The unit.
let unit: Unit = { tier: "x" }

/// The names.
export fn names() -> [String] { return ["a"] }

emit cpp { out: "@features/p", namespace: "p", mode: baked }
emit json { out: "out/" }
`
	hintTypesSrc = `package p

/// A tone.
enum Tone { red, blue }

/// A potion.
record Potion {
  /// Hit points restored.
  heal: Int

  /// What it heals of missing.
  export fn healFor(self, missing: Int) -> Int { return min(heal, missing) }
}

/// Doubles.
export fn twice(n: Int) -> Int { return n * 2 }

emit cpp { out: "@features/p", namespace: "p", mode: types }
`
	hintReloadSrc = `package p

/// A kind.
enum K { one, two }

/// A mark.
record Mark {
  /// Its weight.
  weight: Int
}

/// The tiers.
let tiers: table Mark = {
  x { weight: 1 }
}

/// A payload.
type Pay(k: K) = match k { one => Int, two => ref tiers }

/// A thing.
record Thing {
  /// Its kind.
  k: K
  /// Its payload.
  pay: Pay(k)
  /// A key.
  key: input String? from env "KEY"
}

/// The thing.
let thing: Thing = { k: one, pay: 1 }

/// The live mark.
@reload
let live: Mark = { weight: 3 }

emit cpp { out: "@features/p", namespace: "p", mode: data }
emit json { out: "out/" }
`
	hintBranchSrc = `package p

/// A kind.
enum K { one, two }

/// An event.
record Ev {
  /// Its kind.
  k: K
}

/// The events.
let evs: table Ev = {
  x { k: one }
}

/// A payload read through an event.
type R(e: Ev) = match e.k { one => Int, _ => String }

/// A thing.
record Thing {
  /// Its event.
  ev: ref evs
  /// Its payload.
  r: R(ev)
}

/// The thing.
let thing: Thing = { ev: "x", r: 1 }

emit cpp { out: "@features/p", namespace: "p", mode: data }
emit json { out: "out/" }
`
	hintDefinesSrc = `package p

local let monsters = load.defines("@features/p/monsters.h")

/// A book.
record Book {
  /// Its monster.
  monster: ref monsters

  /// Twice its rank.
  export fn doubled(self) -> Int { return 2 }
}

/// The books.
let books: table Book = {
  one { monster: "M" }
}

emit cpp { out: "@features/p", namespace: "p", mode: data }
emit json { out: "out/" }
`
	hintNamespaceSrc = `package p

/// A tone.
enum Tone { red, blue }

emit cpp { out: "@features/p", namespace: "world::inner", mode: types }
`
	hintLibSrc = `package lib

/// A book.
record Book {
  /// Its rank.
  rank: Int
}

emit cpp { out: "@features/lib", namespace: "lib", mode: types }
`
	hintRowSrc = `package p

import lib { Book }

/// The books.
let kinds: table Book = {
  one { rank: 1 }
}

emit cpp { out: "@features/p", namespace: "p", mode: data }
emit json { out: "out/" }
`
	hintEnumTablesSrc = `package p

/// A stat.
enum Stat { atk = "ATK", def }

/// Words with the wires as names.
enum Plain { up, down }

/// A shape.
variant Shape {
  /// A dot.
  dot
  /// A circle.
  circle {
    /// Its radius.
    r: Int
  }
}

/// A tier.
record Tier {
  /// Its rank.
  rank: Int
}

/// The tiers.
let tiers: table Tier = {
  x { rank: 1 }
  zed { rank: 2 }
}

/// A use of everything.
record Holder {
  /// Its stat.
  s: Stat
  /// Its word.
  q: Plain
  /// Its shape.
  sh: Shape
}

/// The holder.
let holder: Holder = { s: atk, q: up, sh: dot }

emit cpp { out: "@features/p", namespace: "p", mode: baked }
emit json { out: "out/" }
`
	canonFile    = "p/p.canon"
	hintPackage  = "package p\n"
	hintImport   = "package p\n\nimport a { Word }\n\n/// Uses a type of a.\nrecord Uses {\n  /// Its word.\n  w: Word\n}\n"
	hintAccessor = `title: String`
	hintStable   = `rank: Int @stable`
	hintTiersJ   = `{ "x": { "title": "a", "rank": 1 } }`
	hintUnitJ    = `{ "tier": "x" }`
)

// hintPrograms are the families of E8005 names (CODEGEN.md §7.9): a field's storage, ref storage and lookups, a fn's pure function, vector names and parameter, the members every container, entry, class and access struct has, the decoders, the conformance names, snapshot, store and inputs, a dependent type's arms, a namespace segment and the row class; rows of the other variants stay as they were.
var hintPrograms = []hintProgram{
	{files: map[string]string{canonFile: hintDataSrc, "p.tiers.json": hintTiersJ}, rows: []hintRow{
		derivedRow("title_", "p.Tier.title", "title", "heading"),
		defineRow("GetTitle", "p.Tier.title", hintAccessor, hintAccessor+` @cpp(name: "Heading")`),
		derivedRow("FindByRank", "p.Tier.rank", "rank", "level"),
		derivedRow("byRank_", "p.Tier.rank", "rank", "level"),
		derivedRow("byRankKeys_", "p.Tier.rank", "rank", "level"),
		fixedRow("GetId", "p.Tier"),
		fixedRow("Decode", "p"),
	}},
	{files: map[string]string{canonFile: hintBakedSrc, "p.tiers.json": `{ "x": { "rank": 1 }, "zed": { "rank": 2 } }`, "p.unit.json": hintUnitJ}, rows: []hintRow{
		fixedRow("Get", "p.tiers"), fixedRow("Get", "p"), fixedRow("Data", "p"), fixedRow("Build", "p"), fixedRow("PAccess", "p"),
		fixedRow("to_json", "p"), fixedRow("GetRetired", "p.Tier"),
		derivedRow("tier_ref_", "p.Unit.tier", "tier:", "slot:", `"tier"`, `"slot"`),
		derivedRow("names", "p.names", "fn names", "fn labels"),
		idRow("zed", "p.tiers.zed", "zed", "zee"),
	}},
	{files: map[string]string{canonFile: hintTypesSrc}, rows: []hintRow{
		derivedRow("Potion_healFor", "p.Potion.healFor", "healFor", "mendFor"),
		derivedRow("PotionHealForVector", "p.Potion.healFor", "healFor", "mendFor"),
		derivedRow("kPotionHealFor", "p.Potion.healFor", "healFor", "mendFor"),
		derivedRow("missing", "p.Potion.healFor.missing", "missing", "lacking"),
		derivedRow("kTwice", "p.twice", "twice", "dbl"),
		derivedRow("TwiceVector", "p.twice", "twice", "dbl"),
		derivedRow("heal_", "p.Potion.heal", "heal:", "cure:", "min(heal", "min(cure"),
		defineRow("GetHeal", "p.Potion.heal", "heal: Int", `heal: Int @cpp(name: "Cure")`),
		defineRow("Twice", "p.twice", "export fn twice", "@cpp(name: \"Doubler\")\nexport fn twice"),
		fixedRow("want", "p.Potion.healFor"), fixedRow("want", "p.twice"), fixedRow("g_code", "p.conformance"),
		fixedRow("Capture", "p.conformance"), fixedRow("RunPConformance", "p"), fixedRow("ToName", "p"),
		fixedRow("Decode", "p.Potion"), fixedRow("Decode", "p"), fixedRow("Parse", "p.Potion"),
	}},
	{files: map[string]string{canonFile: hintReloadSrc, "p.thing.json": `{ "k": "one", "pay": 1 }`, "p.live.json": `{ "weight": 3 }`, "p.tiers.json": `{ "x": { "weight": 1 } }`}, rows: []hintRow{
		derivedRow("key_", "p.Thing.key", "key:", "token:"),
		derivedRow("AsOne", "p.Pay.one", "one", "uno"),
		derivedRow("Pay_One", "p.Pay.one", "one", "uno"),
		fixedRow("GetBranch", "p.Pay"), fixedRow("PSnapshot", "p"), fixedRow("PStore", "p"), fixedRow("current_", "p"),
		fixedRow("LoadInputs", "p"), fixedRow("PInputs", "p"),
	}},
	{files: map[string]string{canonFile: hintBranchSrc, "p.thing.json": `{ "ev": "x", "r": 1 }`, "p.evs.json": `{ "x": { "k": "one" } }`}, rows: []hintRow{
		derivedRow("kThingRBranches", "p.Thing.r", "r: R(ev)", "pay: R(ev)", `"r"`, `"pay"`, ", r: 1", ", pay: 1"),
	}},
	{files: map[string]string{canonFile: hintDefinesSrc, "p.books.json": `{ "one": { "monster": "M" } }`}, rows: []hintRow{
		derivedRow("doubled_", "p.Book.doubled", "doubled", "twofold"),
		defineRow("Doubled", "p.Book.doubled", "export fn doubled", "@cpp(name: \"Twofold\")\n  export fn doubled"),
		derivedRow("monster_", "p.Book.monster", "monster:", "beast:", `"monster"`, `"beast"`),
		derivedRow("monsterValue_", "p.Book.monster", "monster:", "beast:", `"monster"`, `"beast"`),
		defineRow("GetMonsterValue", "p.Book.monster", "monster: ref monsters", `monster: ref monsters @cpp(name: "Beast")`),
		derivedRow("kMonstersDefines", "p.monsters", "monsters", "creatures"),
	}},
	{files: map[string]string{canonFile: hintNamespaceSrc}, rows: []hintRow{
		fixedRow("world", "p"), fixedRow("inner", "p"), fixedRow("ToName", "p"),
	}},
	{files: map[string]string{canonFile: hintRowSrc, "lib/lib.canon": hintLibSrc, "p.kinds.json": `{ "one": { "rank": 1 } }`}, rows: []hintRow{
		defineRow("BookRow", "lib.Book", "record Book {", `record Book @cpp(name: "Tome") {`),
	}},
	{files: map[string]string{canonFile: hintEnumTablesSrc, "p.tiers.json": `{ "x": { "rank": 1 }, "zed": { "rank": 2 } }`}, rows: []hintRow{
		defineRow("kStatNames", "p.Stat", "enum Stat {", `enum Stat @cpp(name: "Attr") {`),
		defineRow("kStatWires", "p.Stat", "enum Stat {", `enum Stat @cpp(name: "Attr") {`),
		defineRow("kStatByWire", "p.Stat", "enum Stat {", `enum Stat @cpp(name: "Attr") {`),
		defineRow("kPlainNames", "p.Plain", "enum Plain {", `enum Plain @cpp(name: "Flat") {`),
		defineRow("kShapeKindNames", "p.Shape", "variant Shape {", `variant Shape @cpp(name: "Form") {`),
		defineRow("kShapeKindByWire", "p.Shape", "variant Shape {", `variant Shape @cpp(name: "Form") {`),
		defineRow("kTierIdWires", "p.Tier", "record Tier {", `record Tier @cpp(name: "Rank") {`),
		defineRow("kTierIdByWire", "p.Tier", "record Tier {", `record Tier @cpp(name: "Rank") {`),
	}},
}
