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

.PHONY: check fmt-check vet test goldens-vet goldens-check diag-check vm-check audit-self audit-check audit audit-tighten scope

# CANON_REQUIRE_CXX turns a missing C++ compiler or nlohmann/json header (internal/testkit/cxx)
# from a silent test skip into a failure, and is inherited by every prerequisite below: make
# check must not pass green having skipped every C++ compile test for want of a toolchain.
check: export CANON_REQUIRE_CXX=1
check: fmt-check vet test goldens-vet goldens-check diag-check vm-check audit-self audit-check

fmt-check:
	@out="$$(gofmt -l $(GO_FILES))"; if [ -n "$$out" ]; then echo "gofmt -l: not formatted:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

test:
	go test ./...

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
# listed (findings.txt, MANIFEST and go.mod excepted; also examples/pipeline/expected/potion.
# view.json by its exact path, held until M3 wires gen/view, VIEWMODEL.md's golden link and
# GEN-01, DECISIONS 190). internal/testkit/golden's TestExamples then rebuilds every buildable
# example into a temporary copy of examples/ with every outside root redirected and diffs it,
# failing also when the build wrote an output the MANIFEST does not list (IMPLEMENTATION-PLAN.md
# §7.1, DECISIONS 201).
goldens-check:
	@status=0; for m in $$(find examples -path '*/expected/MANIFEST' | sort); do \
	  d=$$(dirname $$m); \
	  for g in $$(awk 'NF != 2 { print "BAD:" NR; next } { print $$2 }' $$m); do \
	    if [ ! -f "$$d/$$g" ]; then echo "goldens-check: $$m names $$g, which does not exist"; status=1; fi; \
	  done; \
	  for f in $$(cd $$d && find . -type f ! -name findings.txt ! -name MANIFEST ! -name go.mod | sed 's|^\./||' | sort); do \
	    if [ "$$d/$$f" = "examples/pipeline/expected/potion.view.json" ]; then continue; fi; \
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

# Project-size report: git-tracked code lines, blanks and comments excluded (tools/scope.sh).
scope:
	@bash tools/scope.sh
