.DEFAULT_GOAL := check
GO ?= go

.PHONY: fmt fmt-check build vet test race check dev-db

fmt:
	$(GO) fmt ./...

fmt-check:
	@files=$$(gofmt -l .) || exit $$?; \
	if [ -n "$$files" ]; then \
		printf 'Arquivos fora do padrão gofmt:\n%s\nExecute make fmt para corrigir.\n' "$$files"; \
		exit 1; \
	fi

build:
	$(GO) build ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

# Requires a C compiler on Linux; SQLite remains the pure-Go test driver.
race:
	CGO_ENABLED=1 $(GO) test -race -count=1 ./...

dev-db:
	KRIOS_DEV_DB_PATH="$(CURDIR)/dev.db" $(GO) test -run '^TestPrepareDevelopmentDatabase$$' -count=1 -v ./tests

check: fmt-check build vet test
