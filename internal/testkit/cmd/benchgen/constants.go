package main

import "os"

// Flags, usage and exit codes (mirrors internal/testkit/cmd/fixturegen).
const (
	flagSeed   = "seed"
	flagTarget = "out"
	flagN      = "n"
	usageSeed  = "the splitmix64 seed"
	usageOut   = "the directory to write the project into; refused if it already exists and holds files"
	usageN     = "the number of Item entries (IMPLEMENTATION-PLAN.md §7.6)"
	exitOK     = 0
	exitFail   = 1
	exitUsage  = 2
	fmtFail    = "benchgen: %v\n"
)

// jsonNull is an optional field's absent value on the JSON wire.
const jsonNull = "null"

const (
	filePerm os.FileMode = 0o600
	dirPerm  os.FileMode = 0o750
)

const defaultN = 7000

const boolChoices = 2

// monsterDivisor and categoryGroupDivisor scale down from n, keeping the 7,000 : 500 ratio.
const (
	monsterDivisor       = 14
	categoryGroupDivisor = 50
)

// optionalOneIn draws an optional field's presence: absent one draw in this many.
const optionalOneIn = 3

// Case field bounds (schema.go), one name per meaning even where two share a value.
const (
	combatStatBound   = 999
	durabilityBound   = 100
	weightClassBound  = 5
	levelBound        = 200
	statCostBound     = 500
	socketBound       = 6
	upgradeBound      = 20
	vendorPriceBound  = 100_000
	comboBound        = 10
	tierBound         = 10
	sortBound         = 1_000
	costBound         = 1_000_000
	weightFloatBound  = 500.0
	stackBound        = 9_999
	stackDefault      = 99
	encumbranceBound  = 99.0
	floatUnit         = 1.0
	critMultiplierMin = 1.01
	critMultiplierMax = 5.0
	floatNegOne       = -1.0
	floatRange50      = 50.0
	floatRadius20     = 20.0
	cooldownMaxMs     = 3_600_000
	castTimeMaxMs     = 10_000
	effectMaxMs       = 86_400_000
	keywordsMax       = 5
	bonusStatsMax     = 8
	modsMax           = 6
	hexColorDigits    = 6
	hexBase           = 16
	floatSteps        = 100
	floatPrecision    = 2
	hpBound           = 100_000
)

// identCategories and identMonsters name the two collections a `ref` field may target.
const (
	identCategories = "categories"
	identMonsters   = "monsters"
)

// canonListSep and jsonListSep join a rendered list's elements.
const (
	canonListSep = ", "
	jsonListSep  = ","
)

const iconExt = ".png"

const minDropTier = 3

// itemCodeFmt, categoryCodeFmt and monsterCodeFmt are the padded id formats (model.go).
const (
	itemCodeFmt     = "ITM_%06d"
	categoryCodeFmt = "CATEGORY_%06d"
	monsterCodeFmt  = "MON_%05d"
)

// qualityTypeName and elementTypeName are Canon enum type text (schema.go).
const (
	qualityTypeName = "Quality"
	elementTypeName = "Element"
)

// fieldItemCode and fieldDisplayName are the Item record's two identifying scalar fields.
const (
	fieldItemCode    = "itemCode"
	fieldDisplayName = "displayName"
)
