# The gate every change passes (DECISIONS.md, decision 25). `make check` runs, in order:
# formatting, vet, tests, the generated goldens, the diagnostics registry, then the code audit
# (tools/audit) on itself and on the repository (IMPLEMENTATION-PLAN.md §12.2).
# Tip: GOTOOLCHAIN=local make check never downloads a Go toolchain.
# examples/go.mod holds no package: it only keeps the goldens out of the compiler module, so
# nothing vets it; each golden Go module has its own go.mod and is vetted by goldens-vet.

AUDIT_DIR   := tools/audit
AUDIT       := cd $(AUDIT_DIR) && go run .
# Every Go module holding generated goldens: vetted and tested in place, never formatted or
# audited (the compiler writes them; goldens-check diffs them).
GOLDEN_MODS := $(patsubst %/go.mod,%,$(shell find examples -path '*/expected/*' -name go.mod 2>/dev/null))
# Hand-written Go files: everything but goldens, generated files, fixtures and testdata.
GO_FILES     = $(shell find . -name '*.go' -not -path './examples/*/expected/*' \
                 -not -path './examples/_fixtures/*' -not -path '*/testdata/*' \
                 -not -path './.claude/*' -not -name '*.gen.go')

.PHONY: check fmt-check vet test stress-short goldens-vet goldens-check diag-check vm-check audit-self audit-check audit audit-tighten scope

# CANON_REQUIRE_CXX turns a missing C++ compiler or nlohmann/json header (internal/testkit/cxx)
# from a silent test skip into a failure, and is inherited by every prerequisite below: make
# check must not pass green having skipped every C++ compile test for want of a toolchain.
check: export CANON_REQUIRE_CXX=1
check: fmt-check vet test stress-short goldens-vet goldens-check diag-check vm-check audit-self audit-check

fmt-check:
	@out="$$(gofmt -l $(GO_FILES))"; if [ -n "$$out" ]; then echo "gofmt -l: not formatted:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

test:
	go test -timeout 30m ./...

# IMPLEMENTATION-PLAN.md §6 M4 item 6: 8 readers, 1 editor and 1 watcher on one project on the
# OS's file system, under -race, 2 s here; `make stress` runs it for STRESS_TIME (60 s).
stress-short:
	TMPDIR=/var/tmp go test -race -count=1 -run '^TestStress$$' ./api

# A golden module with a smoke test (internal/testkit/golden/testdata/smoke/<example>/, DECISIONS
# 201) is copied to a temporary directory with the smoke test added, so expected/ keeps holding
# only compiler output; one without is vetted and tested in place. Assumes one golden Go module
# per example: the smoke directory is named after the example two levels above expected/'s go.mod.
# A data-mode example's own top-level *.json (examples/<ex>/expected/*.json, e.g. pipeline's
# potions.json) is copied into the smoke module's data/ too, so the smoke test loads the real
# golden instead of a hand-kept copy; a baked example's data already sits inside its module.
goldens-vet:
	@for m in $(GOLDEN_MODS); do \
	  echo "golden module $$m"; \
	  smoke="internal/testkit/golden/testdata/smoke/$$(basename $$(dirname $$(dirname $$m)))"; \
	  if [ -d "$$smoke" ]; then \
	    tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; \
	    cp -r "$$m/." "$$tmp" && mkdir -p "$$tmp/smoke" && cp -r "$$smoke/." "$$tmp/smoke" && \
	      for j in $$(find "$$(dirname $$m)" -maxdepth 1 -name '*.json'); do \
	        mkdir -p "$$tmp/smoke/data" && cp "$$j" "$$tmp/smoke/data/"; \
	      done && \
	      (cd "$$tmp" && go vet ./... && go test -race ./...); \
	    status=$$?; rm -rf "$$tmp"; trap - EXIT; \
	    [ $$status -eq 0 ] || exit $$status; \
	  else \
	    (cd $$m && go vet ./... && go test -race ./...) || exit 1; \
	  fi; \
	done

# Every expected/MANIFEST line names a golden that exists, and every file of that expected/ is
# listed (findings.txt, MANIFEST and go.mod excepted). internal/testkit/golden's TestExamples
# then rebuilds every buildable example into a temporary copy of examples/ with every outside
# root redirected and diffs it, failing also when the build wrote an output the MANIFEST does
# not list (IMPLEMENTATION-PLAN.md §7.1, DECISIONS 201).
goldens-check:
	@status=0; for m in $$(find examples -path '*/expected/MANIFEST' | sort); do \
	  d=$$(dirname $$m); \
	  for g in $$(awk 'NF != 2 { print "BAD:" NR; next } { print $$2 }' $$m); do \
	    if [ ! -f "$$d/$$g" ]; then echo "goldens-check: $$m names $$g, which does not exist"; status=1; fi; \
	  done; \
	  for f in $$(cd $$d && find . -type f ! -name findings.txt ! -name MANIFEST ! -name go.mod | sed 's|^\./||' | sort); do \
	    if ! awk '{ print $$2 }' $$m | grep -qx "$$f"; then echo "goldens-check: $$d/$$f is not listed in $$m"; status=1; fi; \
	  done; \
	done; \
	go test -race ./internal/testkit/golden/... -run TestExamples -v || status=1; \
	exit $$status

# internal/diag/codes.go and its generated test table equal what diaggen generates from
# spec/ERRORS.md into a temporary directory, and the runtime helper texts under internal/gen
# use only the pairs of ERRORS.md §1.6 (IMPLEMENTATION-PLAN.md §12.2 step 5).
diag-check:
	@tmp=$$(mktemp -d) && trap 'rm -rf "$$tmp"' EXIT && \
	  go run ./internal/diag/cmd/diaggen -out "$$tmp" -runtime internal/gen && \
	  diff -u internal/diag/codes.go "$$tmp/codes.go" && \
	  diff -u internal/diag/codes_test.go "$$tmp/codes_test.go"

# api/vm/vm.gen.go equals what vmgen generates from spec/viewmodel.schema.json into a temporary
# directory (API.md R10).
vm-check:
	@tmp=$$(mktemp -d) && trap 'rm -rf "$$tmp"' EXIT && \
	  go run ./api/vm/internal/vmgen -out "$$tmp" && \
	  diff -u api/vm/vm.gen.go "$$tmp/vm.gen.go"

# The audit tool gates itself first: its own vet, tests and baseline (tools/audit/.sovaudit).
audit-self:
	cd $(AUDIT_DIR) && go vet ./... && go test ./...
	$(AUDIT) check --repo . --quiet

audit-check:
	$(AUDIT) check --repo ../.. --quiet

# The census: every rule with its count, zeros included, and the first findings of each.
audit:
	$(AUDIT) audit --repo ../..

# After a cleanup: lower .sovaudit/baseline.tsv to what is left (it never grows).
audit-tighten:
	$(AUDIT) baseline --tighten --repo ../..

# Real-data job (IMPLEMENTATION-PLAN.md §6 M3 item 7, §7.3; DECISIONS 29): `canon check` on the
# examples that read `@resource`, against the real Resource tree in the git-ignored
# testdata-real/. Non-gating, never part of `check`; findings go to
# testdata-real/realdata/findings.txt for review, not to the exit code.
.PHONY: check-real
check-real:
	@bash tools/check-real.sh

# DECISIONS 200: the generated-program suites at nightly size, under the memory cap, once per
# seed of PROGEN_SEEDS (decision log A3: larger N, several seeds; seeds a million apart so no
# two runs share a case); a crash is reported with its seed and the suite goes on; -progen.keep
# writes the shrunk counterexamples. Every seed runs even when one fails. TestTyped caps its
# cases at 1000 (each compiles and tests a Go module), TestMetamorphic at 5000. Cost, measured
# at N=5000 on 4 cores: about 10 min a seed (TestTyped 7 of them); at N=10000 an estimated 13 min.
PROGEN_N     ?= 10000
PROGEN_SEEDS ?= 1 1000001 2000001
.PHONY: progen-nightly
progen-nightly:
	@status=0; for seed in $(PROGEN_SEEDS); do \
	  echo "progen-nightly: seed $$seed"; \
	  systemd-run --user --scope -q -p MemoryMax=3G env GOTOOLCHAIN=local go test -count=1 -timeout 0 \
	    -run 'TestMutations|TestGrammar|TestCorruption|TestTyped|TestMetamorphic' ./internal/testkit/progen \
	    -progen.n $(PROGEN_N) -progen.seed $$seed -progen.keep || status=1; \
	done; exit $$status

# A zero-findings check of the benchmark project at its default 7,000-entry size
# (IMPLEMENTATION-PLAN.md §7.6), too heavy for `make check` (`TestCheckClean` there runs at the
# PR size, 1,000): opt-in only, capped like progen-nightly above. The NFR-01 time and memory
# targets are measured by `bench-edit` below (§7.6).
.PHONY: bench
bench:
	systemd-run --user --scope -q -p MemoryMax=6G env GOTOOLCHAIN=local go test -count=1 -timeout 0 \
	  -run TestCheckCleanDefaultN -v ./internal/testkit/cmd/benchgen -benchgen.full

# Project-size report: git-tracked code lines, blanks and comments excluded (tools/scope.sh).
scope:
	@bash tools/scope.sh

# IMPLEMENTATION-PLAN.md §6 M4 item 6 at its stated length: the stress test for STRESS_TIME,
# capped like `bench`.
STRESS_TIME ?= 60s
.PHONY: stress
stress:
	systemd-run --user --scope -q -p MemoryMax=3G env GOTOOLCHAIN=local TMPDIR=/var/tmp go test -race -count=1 -timeout 0 \
	  -run '^TestStress$$' -v ./api -stress.duration $(STRESS_TIME)

# IMPLEMENTATION-PLAN.md §6 M4 item 3, §7.7: the minimal-write fuzz (API.md M6) for
# FUZZ_EDIT_TIME on every example and on a benchmark project of FUZZ_EDIT_BENCH_N entries
# (0: the examples alone); benchgen writes canonical JSON sources (FMT-02, DECISIONS 12). Opt-in;
# the whole run (benchgen, the fuzz and its FUZZ_EDIT_WORKERS workers) under one 6G cap.
# The corpus is the seeds and testdata/fuzz alone, in a fuzz cache of the run: the user cache
# grows with every run and is all replayed as baseline first. No minimizing: past its time Go
# kills and restarts the worker, which reopens the benchmark (log-2026-09-29 M4 P20).
# §7.6, no silent pass: fuzz-edit-judge fails unless baseline coverage completes and the
# coordinator seeded (and, with the benchmark, proved its seeds); then per minute fuzzed at least
# FUZZ_EDIT_MIN_EXECS inputs must run past baseline and each project get FUZZ_EDIT_MIN_APPLIED
# edits applied past its seeds (the -edit.tally file), Set, Add and Remove each at least once
# on the benchmark. The floors are 11-14% of the execs and 15-19% of the benchmark's edits two
# 10-minute runs gave on a 24-core host at load 7-24.
FUZZ_EDIT_TIME        ?= 10m
FUZZ_EDIT_BENCH_N     ?= 7000
FUZZ_EDIT_WORKERS     ?= 2
FUZZ_EDIT_MIN_EXECS   ?= 2000
FUZZ_EDIT_MIN_APPLIED ?= 200
.PHONY: fuzz-edit fuzz-edit-judge
fuzz-edit:
	systemd-run --user --scope -q -p MemoryMax=6G env GOTOOLCHAIN=local TMPDIR=/var/tmp \
	  N=$(FUZZ_EDIT_BENCH_N) T=$(FUZZ_EDIT_TIME) W=$(FUZZ_EDIT_WORKERS) bash -c '\
	  set -o pipefail; \
	  dir=$$(mktemp -d /var/tmp/canon-fuzz-edit-XXXXXX); trap "rm -rf \"$$dir\"" EXIT; bench=""; \
	  if [ "$$N" != 0 ]; then \
	    go run ./internal/testkit/cmd/benchgen -seed 1 -n "$$N" -out "$$dir/bench" || exit 1; \
	    bench="-edit.bench=$$dir/bench"; \
	  fi; \
	  go test -v -count=1 -timeout 0 -run "^$$" -fuzz "^FuzzMinimalWriteAll$$" -fuzztime "$$T" -fuzzminimizetime 0 \
	    -parallel "$$W" ./internal/edit $$bench -edit.tally="$$dir/tally" -test.fuzzcachedir="$$dir/cache" 2>&1 | tee "$$dir/log" || exit 1; \
	  $(MAKE) --no-print-directory fuzz-edit-judge FUZZ_EDIT_LOG="$$dir/log" FUZZ_EDIT_TALLY="$$dir/tally"'

# Judges a fuzz-edit run from its go test log and tally, both named: a run kept aside, or a mock.
fuzz-edit-judge:
	@N=$(FUZZ_EDIT_BENCH_N) LOG="$(FUZZ_EDIT_LOG)" TALLY="$(FUZZ_EDIT_TALLY)" \
	  MIN_EXECS=$(FUZZ_EDIT_MIN_EXECS) MIN_APPLIED=$(FUZZ_EDIT_MIN_APPLIED) bash -c '\
	  fail() { echo "fuzz-edit: FAIL: $$*"; exit 1; }; \
	  projects=examples; [ "$$N" = 0 ] || projects="examples bench"; \
	  base=$$(sed -n "s/^fuzz: .*: \([0-9]*\)\/[0-9]* completed, now fuzzing.*/\1/p" "$$LOG"); \
	  [ -n "$$base" ] || fail "baseline coverage never completed"; \
	  [ "$$N" = 0 ] || grep -q "benchmark seeds applied by operation" "$$LOG" || fail "the benchmark seeds were never proved"; \
	  last=$$(grep "^fuzz: elapsed: .*, execs: " "$$LOG" | tail -n 1); \
	  el=$$(echo "$$last" | sed -n "s/^fuzz: elapsed: \([^,]*\),.*/\1/p"); \
	  echo "$$el" | grep -qx "\([0-9][0-9]*h\)\{0,1\}\([0-9][0-9]*m\)\{0,1\}[0-9][0-9]*\(\.[0-9]*\)\{0,1\}s" \
	    || fail "cannot read the elapsed time \"$$el\""; \
	  h=$$(echo "$$el" | sed -n "s/^\([0-9]*\)h.*/\1/p"); \
	  m=$$(echo "$$el" | sed -n "s/^\([0-9]*h\)\{0,1\}\([0-9]*\)m.*/\2/p"); \
	  s=$$(echo "$$el" | sed -n "s/^\(.*[hm]\)\{0,1\}\([0-9]*\)\(\.[0-9]*\)\{0,1\}s$$/\2/p"); \
	  secs=$$(( $${h:-0} * 3600 + $${m:-0} * 60 + $${s:-0} )); \
	  [ "$$secs" -gt 0 ] || fail "cannot read the elapsed time \"$$el\": no second fuzzed"; \
	  execs=$$(echo "$$last" | sed -n "s/.*, execs: \([0-9]*\) .*/\1/p"); \
	  new=$$(echo "$$last" | sed -n "s/.*new interesting: \([0-9]*\) .*/\1/p"); \
	  floor() { local f=$$(( $$1 * secs / 60 )); echo $$(( f > 0 ? f : 1 )); }; \
	  count() { local c; c=$$(grep -c -x "$$1" "$$TALLY" 2>/dev/null); echo "$${c:-0}"; }; \
	  seeded=$$(count "[a-z]* seed [A-Za-z]*"); extra=$$(( base > seeded ? base - seeded : 0 )); \
	  fuzzed() { echo $$(( $$(count "$$1 applied $$2") - $$(count "$$1 seed $$2") - extra )); }; \
	  past=$$(( $${execs:-0} - base )); want=$$(floor "$$MIN_EXECS"); \
	  echo "fuzz-edit: $$el fuzzed; baseline $$base inputs; $$past execs past it (floor $$want); $${new:-0} new interesting"; \
	  [ "$$past" -ge "$$want" ] || fail "$$past execs past baseline, under $$want"; \
	  want=$$(floor "$$MIN_APPLIED"); \
	  for p in $$projects; do \
	    [ "$$(count "$$p seed [A-Za-z]*")" -gt 0 ] || fail "$$p: no seed tallied, the coordinator did not seed"; \
	    n=$$(fuzzed "$$p" "[A-Za-z]*"); \
	    echo "fuzz-edit: $$p: $$n edits applied past the seeds (floor $$want)"; \
	    [ "$$n" -ge "$$want" ] || fail "$$p: $$n edits applied past the seeds, under $$want"; \
	  done; \
	  [ "$$N" = 0 ] || for k in Set Add Remove; do \
	    n=$$(fuzzed bench "$$k"); echo "fuzz-edit: bench: $$n $$k applied past the seeds"; \
	    [ "$$n" -gt 0 ] || fail "bench: no $$k applied past the seeds"; \
	  done'

# IMPLEMENTATION-PLAN.md §6 M4 item 4, §7.6: NFR-01 on a benchmark project of BENCH_EDIT_N
# entries and on the examples, printed with the machine. Gated: cold check and its RSS per
# project, Edit and Evaluate p95, view model size; the warm check is reported, not gated (log
# M4 U7b). Opt-in, not part of `check`.
BENCH_EDIT_N ?= 7000
.PHONY: bench-edit
bench-edit:
	systemd-run --user --scope -q -p MemoryMax=6G env GOTOOLCHAIN=local TMPDIR=/var/tmp go test -count=1 -timeout 0 \
	  -run '^TestBenchEdit$$' -v ./internal/testkit/cmd/benchgen -benchgen.edit $(BENCH_EDIT_N)

# IMPLEMENTATION-PLAN.md §6 M5: diagnostics for an edited entry file of the benchmark project
# published within 500 ms of the last change (p95, debounce included). Opt-in, not part of `check`.
.PHONY: bench-lsp
bench-lsp:
	systemd-run --user --scope -q -p MemoryMax=6G env GOTOOLCHAIN=local TMPDIR=/var/tmp go test -count=1 -timeout 0 \
	  -run '^TestLatency$$' -v ./internal/lsp -lsp.bench $(BENCH_EDIT_N)
