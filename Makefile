.DEFAULT_GOAL := check

.PHONY: fmt fmt-check build vet test check

fmt:
	go fmt ./...

fmt-check:
	@files=$$(gofmt -l .) || exit $$?; \
	if [ -n "$$files" ]; then \
		printf 'Arquivos fora do padrão gofmt:\n%s\nExecute make fmt para corrigir.\n' "$$files"; \
		exit 1; \
	fi

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./...

check: fmt-check build vet test
