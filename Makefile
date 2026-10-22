GO := $(if $(wildcard .tools/go/bin/go),.tools/go/bin/go,go)

.PHONY: all setup web build test check run serve dev clean
all: build
setup:
	bash scripts/bootstrap-go.sh
	$(MAKE) dependencies

.PHONY: dependencies
dependencies:
	$(GO) mod download
	cd web && pnpm install --frozen-lockfile

web:
	cd web && pnpm build
build: web
	mkdir -p bin
	$(GO) build -trimpath -o bin/hilife .
test:
	$(GO) test -race ./...
check:
	$(GO) vet ./...
	cd web && pnpm check
run:
	$(GO) run .
serve:
	$(GO) run . web
dev:
	bash scripts/dev.sh
clean:
	$(GO) clean
