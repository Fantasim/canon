# ADR-0009 — `check.Info.BrokenViews`: which views hold an error

- Date: 2026-09-29
- Status: accepted (orchestrator, under IMPLEMENTATION-PLAN §4's review rule: `check.Info` is a
  frozen contract, §4.7; `spec-reviewer` reviews the diff as the consumers' proxy — views, i18n)

## Context

VIEWMODEL J4 leaves a `view` holding an error out of `views`, its templates unrendered, and its keys
out of the catalogue (I18N F4). `views` decided it from error spans in the bag's findings, and the
i18n unit re-implemented the same walk. Both read `diag.Bag.Findings()`, which is truncated (every
bag has a limit, `DefaultMaxFindings` or `--max-findings`): the model and the catalogue would change
with the limit (API F7). `Info.Broken` is keyed by objects; a view is not one.

## Decision

- `check.Info` gains `BrokenViews map[*syntax.ViewDecl]bool` (additive): check marks a view when it
  reports an error while resolving or typing it (its items, properties, templates, translations'
  interpolations are separate files and not included). Views-owned findings (E16xx, reported later by
  views/rules) do not mark it; rules skip what check marked.
- One predicate, exported by `check` (e.g. `check.ViewBroken(info, d)`: the mark, or a parser
  recovery node in the view), used by views (shape) and i18n; no span walk over findings anywhere.

## Consequences

The view model and the catalogue no longer depend on the findings limit. `views/shape.Errors` and
i18n's `broken.go` span logic go. Consumers that switch over `Info` fields are unaffected.
