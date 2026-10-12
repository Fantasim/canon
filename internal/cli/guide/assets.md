# Assets

An asset is a `String` that names a file which must exist. The compiler lists the directory
and checks every asset value at build time, so a runtime never meets a missing icon, model or
sound (TYPES.md §13.4, DECISIONS 19).

## The type

What: `asset(root, ext: [a, b])`. `root` is a string literal naming a directory, written like a
`load` path: relative to the file, or `@root/dir` for a project root (`canon guide syntax`).
`ext` lists the extensions as bare words. No constant, no interpolation: the compiler must know
every directory a package reads before it evaluates anything (`E3704` for a bad argument).
When: a value is the name of a file, however the file is used: icons, models, sounds, shaders.
Not for text read into the model: that is `load`.

```canon art/art.canon
/// Banners with a picture each.
package art

/// A picture file under pics/. The extension must be svg or png.
type Picture = asset("pics", ext: [svg, png])

/// A banner.
record Banner {
  /// Shown title.
  title: String(1..)
  /// Its picture, a path below pics/.
  picture: Picture
}

/// Every banner.
let banners: table Banner = {
  spring { title: "Spring", picture: "spring.svg" }
  winter { title: "Winter", picture: "seasons/winter.svg" }
}
```

```text art/pics/spring.svg
<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"/>
```

```text art/pics/seasons/winter.svg
<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"/>
```

## The value

- The value is the file name, or a path below the root, with `/` separators: `"spring.svg"`,
  `"seasons/winter.svg"`. A `..`, a leading `/` or `\`, an empty segment or `.` is `E3703`.
- The extension is the text after the last `.`, compared exactly with `ext`: `E3702` otherwise.
- The file must exist under the root, with exactly that name, byte for byte, on every platform.
  `Spring.svg` for `spring.svg` is `E3701`, even on a file system that would open it. Legacy data
  with the wrong case is fixed once in the data; the language does not tolerate it.
- An asset is a plain `String` on the wire (`"szIcon": "axe.dds"`). In generated Go, C++ and
  TypeScript it is a `string`; the check happened at build time and runs nowhere else.
  `@json` forms apply as to any string: `@json("szIcon")` keeps the legacy key.
- Two directories need two asset types (`type Icon = asset(...)`, `type Sound = asset(...)`).

## In the studio

The view model gives the field a `file` control, `{"kind": "file", "root": "art/pics",
"ext": ["svg", "png"]}`, with the root as a project path, and lists each root under `assets`
(VIEWMODEL.md §12.5, §12.9). The studio shows a thumbnail and a file picker limited to the root
and the extensions. `preview <asset field>` in a view names the thumbnail of an entry
(`canon guide views-i18n`). Decoding a format such as DDS is the studio's job.

## A root a machine lacks

A root outside the project may be absent on some machines (`optional_roots`, `canon guide
syntax`). Every asset value under it is then `E3705`, naming the root, instead of `E3701`:
`canon check` of the packages that do not read it still works, or place the root with
`project.local.canon`.

## Mistakes

| Mistake | Result |
|---|---|
| `Spring.svg` for `spring.svg` | `E3701`: exact case |
| `spring.SVG` when `ext` says `svg` | `E3702`: the extension is compared exactly |
| a path with `..` or a leading `/` | `E3703` |
| a root that is a constant (`asset(ROOT, ...)`) | `E1116`: the root is a string literal; an interpolated one is `E3704` |
| a plain `String` for a file name | no check: the file may be missing at runtime; use the asset type |
