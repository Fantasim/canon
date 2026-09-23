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

.PHONY: check fmt-check vet test goldens-vet goldens-check diag-check audit-self audit-check audit audit-tighten

check: fmt-check vet test goldens-vet goldens-check diag-check audit-self audit-check

fmt-check:
	@out="$$(gofmt -l $(GO_FILES))"; if [ -n "$$out" ]; then echo "gofmt -l: not formatted:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

test:
	go test ./...

goldens-vet:
	@for m in $(GOLDEN_MODS); do echo "golden module $$m"; (cd $$m && go vet ./... && go test ./...) || exit 1; done

# Every expected/MANIFEST line names a golden that exists, and every file of that expected/
# is listed (findings.txt, MANIFEST and go.mod excepted). The rebuild-and-diff of every example
# joins this target when the compiler can build (M1, IMPLEMENTATION-PLAN.md §7.1).
goldens-check:
	@status=0; for m in $$(find examples -path '*/expected/MANIFEST' | sort); do \
	  d=$$(dirname $$m); \
	  for g in $$(awk 'NF != 2 { print "BAD:" NR; next } { print $$2 }' $$m); do \
	    if [ ! -f "$$d/$$g" ]; then echo "goldens-check: $$m names $$g, which does not exist"; status=1; fi; \
	  done; \
	  for f in $$(cd $$d && find . -type f ! -name findings.txt ! -name MANIFEST ! -name go.mod | sed 's|^\./||' | sort); do \
	    if ! awk '{ print $$2 }' $$m | grep -qx "$$f"; then echo "goldens-check: $$d/$$f is not listed in $$m"; status=1; fi; \
	  done; \
	done; exit $$status

# internal/diag/codes.go equals what diaggen generates from spec/ERRORS.md, and the runtime
# helper texts use only the pairs of ERRORS.md §1.6. Active as soon as diaggen exists (M0).
diag-check:
	@if [ -d internal/diag/cmd/diaggen ]; then \
	  go run ./internal/diag/cmd/diaggen -check -runtime internal/gen; \
	else echo "diag-check: internal/diag/cmd/diaggen does not exist yet (M0)"; fi

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
