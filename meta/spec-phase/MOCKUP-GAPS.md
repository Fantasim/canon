# View gaps found while mocking the studio

Source: the studio mockup (`meta/spec-phase/mockups/studio.html`). Each entry gives the situation, then **the
behaviour the mockup chose**, then the proposed spec sentence. DECISIONS 20 accepts these
behaviours for SPEC §16, with one exception: gap 34 follows DECISIONS 19.

## Controls

1. **Variant cases above 4 (§16.2).** Enums and variant cases share one threshold scale: 2–4
   choices use segmented buttons, 5–10 a select, more than 10 a searchable select.
2. **Optional segmented control.** An optional value shown as segmented buttons gets a final
   "Unset" segment instead of a clear button, whatever its type.
3. **Sets of enums.** `where it.isUnique()` on `[Enum]` marks it as a set in the view model.
   Without it, `[Enum]` is always shown as chips.
4. **Short lists where position matters.** A `[scalar]` with an upper length bound of 6 or less
   is shown as that many inputs in a row, each labelled by its position. One refined to
   `len() == 2 and it[0] <= it[1]` is a min–max range.
5. **TimeOfDay.** Add a `time_of_day` widget, or allow a type-level default widget
   (`widget … default`).
6. **Stepper buttons.** Stepper buttons go on the number input of an `Int` whose range has at most
   20 values. `Float` and `Duration` never get them.
7. **Legacy wire forms.**
   - `@json(int)`: a `Bool` written as 0/1.
   - `@json(bits)`: an `[Enum]` written as a bitmask integer.

## Relevance and grouping

8. **Where "More" starts.** A field is shown if at least 25 % of the case's values set it, or if
   this value sets it. The rest go under "More", in usage order.
9. **Order of sections.**
   1. Named groups, in view order, with all their fields regardless of usage.
   2. An unlabelled section ordered by usage.
   3. "More".
   4. "Unused fields".
10. **Nested kinds.** `canon infer --by a,b` writes nested variants, and usage is recorded per
    innermost case.
11. **Optional fields with a non-`none` default** have three states: default, set, and explicitly
    `none`. The studio shows `none` explicitly and offers "Reset to default" separately from
    "Clear".
12. **Text for `none`.** `field { none: "Any" }` gives a label for `none`, and it is translatable.

## Variants and dependent fields

13. **Changing a variant's case.** Fields with the same name and type are kept, and the rest are
    dropped. The studio lists the dropped fields first and offers Undo.
14. **Views that name case-only fields.** A view may name fields that exist in only one case: the
    group hides when the current case lacks them. `view Variant.case { … }` is also allowed.
15. **The case column in a table** is read-only. The case is changed in the detail panel.
16. **A dependent field whose driver changes.** One atomic edit clears the dependent value, with
    Undo. A field of type `Never` is hidden only once its value is `none`.
17. **Keys of a dependent map** are fixed once added. The key is picked first, then the typed
    editor appears.
18. **Editing `T | "lit"`.** It uses T's control, plus one pinned option per literal.

## Pickers and titles

19. **Define tables without views.** A view may be declared on a loaded define table. When several
    tables are candidates, prefer importing one that has a view.
20. **Picker ranking and limits.**
    1. Title prefix match.
    2. Word prefix match.
    3. Title substring match.
    4. Key match.

    Within each tier, shorter titles come first. The picker shows 60 results and the total count.
21. **Retired entries** are never offered in pickers. Existing references to them show a "retired"
    badge next to `E3502`.
22. **A ref inside a title template** shows the target's `title` if it has a view, else its key.
    `{item.id}` forces the key.
23. **Keys of keyed collections.** The title and subtitle cell shows the key. Keys are read-only in
    cells; a "Rename…" action goes through `refs`.
24. **Methods in views.** A view may name a parameterless method; it is shown like a `show` line.
25. **Collection-valued columns** show the element count, with an optional `plural` label.
26. **Duplicate titles.** When titles collide, the key is appended.
27. **Naming the steps of `ordered_steps`.** A field prop such as `step "Winner {index}"`.

## Widgets, units, findings

28. **Widgets that need sibling values.** The widget signature is `(value: Int, siblings: [Int])`.
29. **Maps from an enum to a record** get one card per entry, titled by `{key}`. "Add" offers the
    missing members. Keys show the Canon name, with the wire value as a subtitle.
30. **`unit` on a list or map** applies to each scalar element or value.
31. **Where a record-level finding goes.** It is shown as a banner at the top of the record, and
    the fields the check reads are highlighted. `check … at <field>` pins it to one field.
32. **Package-level checks over collections.** Authors are guided to the block form, with one
    `warn(x, …)` per element.
33. **Findings on hidden fields.** A field with a finding is always shown.
34. **Asset letter case:** exact match (DECISIONS 19).

## Structure and text

35. **A group holding one record field** is flattened: the group label replaces the record's
    title, and the record's groups become subsections.
36. **The final group** is labelled "Other" (key `_other`). It comes after the named groups and
    before "Unused fields".
37. **`advanced`** groups are collapsed by default; the state is remembered per user.
38. **Placing `show` lines.** `show` is allowed inside groups. When evaluation fails, it shows "—"
    and produces no finding.
39. **Defaults for labels.** Fields without a label are humanized from camelCase. Enum members
    without a view show their Canon name. In French, both count as missing keys.
40. **Fields that admit a single value** (`where it == 2`) are read-only.
41. **Showing a Duration** uses its largest exact unit (3600 s is shown as 1 h).
42. **Values without a menu** appear in a "No menu" section, plus a `W16xx` warning.
43. **Filters.**
    - An enum filter is single choice plus "Any" by default; `multi` makes it multi-select.
    - An `Int` filter is a min–max range bounded by the observed values.
    - Optional fields get a "(none)" choice.
44. **Text search above a large collection** reuses the type's `search { }`.
45. **Showing the English fallback** in another language: an "EN" marker on each fallback text,
    and one banner for a package with no file for that language.
46. **Units and number formats in other languages.** Unit suffixes get translation keys. Separators
    follow the language.
47. **When `when` and `show` are re-evaluated:** after each committed edit, not on each keystroke.
48. **Editing while a layer applies.** Edits go to the base sources. Values set by a layer are
    read-only and name the layer.
49. **Parallel legacy fields** (`dwDestParam_i` with `nAdjParamVal_i`) need a `@json(pairs…)`
    mapping, or a view `row` that groups them.
50. **An items example** is missing: add `item.canon` with its view and fr file.

## Examples that contradict the spec

- **C1.** The `view Item` in §16.6 and §16.8 does not match `vocab.canon`'s `Item`.
- **C2.** propItem.json has 127 wire keys and 6,944 items.
- **C3.** §16.6's `when kind is IK1_WEAPON` contradicts "prefer a variant".
- **C4.** farm.canon rejects the real farm_config.json: the 8 Premium models' levels have no
  production fields.
- **C5.** Farm and events use local `items` define tables, so their pickers lose the Item view.
- **C6.** §16.11 and studio.canon differ: Icon members, `weight`, and the widget list.
- **C7.** `Element` has no view in vocab.canon.
- **C8.** farm.view.canon's reference to "§11.8" should point to §17.
- **C9.** Every view uses `menu events`; eventConfig has no view.
- **C10.** Data that an inferred enum would legalize: `WI_WORLD_MADRIGAL` as a weapon type, and an
  unnamed flag bit 8.
