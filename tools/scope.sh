#!/usr/bin/env bash
# Project scope report: how much code this repo actually is.
#
# Counts CODE lines only - blank lines and comments are stripped before the
# count. Only git-tracked files are counted. Generated goldens are reported on
# their own row and group, so the hand-written compiler size stays visible.
# Ported from ../admin/tools/scope.sh and rebucketed for the Canon compiler.
#
# Usage: make scope   (or: bash tools/scope.sh)
set -euo pipefail
export LC_ALL=C

cd "$(dirname "$0")/.."

# One bucket per line: <label>|<comment style>|<group>|<git pathspec>...
# Comment styles: c (// and /* */), hash (#), none.
# Groups roll the buckets up into the summary: app, test, gen, docs, data.
# Order matters - a file lands in the FIRST bucket that matches it, so the
# list below is in claim priority, not display order (rows print by size).
buckets=(
  'Goldens - generated outputs|none|gen|examples/*/expected/*:*/testdata/golden/*:*/testdata/*/out/*:internal/gen/go/testdata/teamboard/*'
  'Fuzz corpus|none|test|*/testdata/fuzz/*'
  'Go - unit tests|c|test|*_test.go'
  'Go - audit tool|c|app|tools/audit/*.go'
  'Go - compiler|c|app|*.go'
  'Codegen runtime templates|c|app|internal/gen/*/runtime/*:internal/gen/cpp/text/*'
  'txtar - test cases|none|test|*.txtar'
  'Canon - example sources|c|test|examples/*.canon'
  'Test fixtures & testdata|none|test|*/testdata/*:examples/_fixtures/*:examples/*'
  'Canon - other sources|c|test|*.canon'
  'Python - agent hooks|hash|app|*.py'
  'Spec - SPEC, CLI, DECISIONS, spec/|none|docs|SPEC.md:CLI.md:DECISIONS.md:spec/*'
  'Docs - DOCTRINE, meta, README|none|docs|*.md'
  'Config - make, mod, CI, audit|hash|data|Makefile:go.mod:*/go.mod:*.yml:*.txt:.gitignore:*.json:*.tsv'
)

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
: > "$tmp/claimed"
: > "$tmp/rows"

count_lines() { # <style> <file>...
  local style=$1; shift
  [ $# -eq 0 ] && { echo 0; return; }
  awk -v style="$style" '
    FNR == 1 { inblock = 0 }
    {
      line = $0
      if (style == "c") {
        while (1) {
          if (inblock) {
            i = index(line, "*/")
            if (i == 0) { line = ""; break }
            line = substr(line, i + 2); inblock = 0
          } else {
            i = index(line, "/*")
            if (i == 0) break
            rest = substr(line, i + 2); line = substr(line, 1, i - 1)
            j = index(rest, "*/")
            if (j == 0) { inblock = 1; break }
            line = line substr(rest, j + 2)
          }
        }
        sub(/\/\/.*/, "", line)
      } else if (style == "hash") {
        sub(/^[ \t]*#.*/, "", line)
      } else if (style == "sql") {
        sub(/--.*/, "", line)
      }
      if (line ~ /[^ \t\r]/) n++
    }
    END { print n + 0 }
  ' "$@"
}

printf '\n  Canon compiler - project scope (git-tracked code, blanks and comments excluded)\n\n'
printf '  %-34s %7s %10s\n' 'Area' 'Files' 'Code'
printf '  %s\n' '--------------------------------------------------------'

total_files=0; total_code=0
declare -A group_code=()
for bucket in "${buckets[@]}"; do
  IFS='|' read -r label style group globs <<< "$bucket"
  IFS=':' read -r -a patterns <<< "$globs"
  # package-lock.json is generated, not authored - never part of the scope.
  git ls-files -z -- "${patterns[@]}" | tr '\0' '\n' | grep -v '^web/package-lock\.json$' | sort > "$tmp/hits"
  comm -23 "$tmp/hits" <(sort "$tmp/claimed") > "$tmp/files"
  cat "$tmp/files" >> "$tmp/claimed"
  mapfile -t files < "$tmp/files"
  [ "${#files[@]}" -eq 0 ] && continue
  code=$(count_lines "$style" "${files[@]}")
  printf '%d\t%s\t%d\n' "$code" "$label" "${#files[@]}" >> "$tmp/rows"
  total_files=$(( total_files + ${#files[@]} ))
  total_code=$(( total_code + code ))
  group_code[$group]=$(( ${group_code[$group]:-0} + code ))
done

sort -rn "$tmp/rows" | while IFS=$'\t' read -r code label nfiles; do
  printf '  %-34s %7d %10d\n' "$label" "$nfiles" "$code"
done

printf '  %s\n' '--------------------------------------------------------'
printf '  %-34s %7d %10d\n\n' 'Total' "$total_files" "$total_code"

printf '  Compiler & tooling (Go, runtime templates, hooks): %d\n' "${group_code[app]:-0}"
printf '  Tests (Go unit, txtar, fixtures, fuzz, examples):   %d\n' "${group_code[test]:-0}"
printf '  Generated goldens (not authored):                   %d\n' "${group_code[gen]:-0}"
printf '  Spec & docs:                                        %d\n' "${group_code[docs]:-0}"
printf '  Config:                                             %d\n\n' "${group_code[data]:-0}"

# Anything tracked that no bucket claimed and that isn't a binary asset.
sort "$tmp/claimed" > "$tmp/claimed.s"
git ls-files | sort | comm -23 - "$tmp/claimed.s" \
  | grep -Ev '(\.(svg|png|jpe?g|webp|webm|mp4|woff2?|ico|sum|pyc|gitkeep|dds)$|^go\.sum$)' > "$tmp/rest" || true
if [ -s "$tmp/rest" ]; then
  printf '  Uncounted text files (%d): %s\n\n' "$(wc -l < "$tmp/rest")" "$(paste -sd' ' "$tmp/rest")"
fi
