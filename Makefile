APP := rebuildable
VERSION ?= 0.1.0
LDFLAGS := -X github.com/jayeshsarvaiya/rebuildable/internal/version.Version=$(VERSION)

.PHONY: build test test-integration lint demo release

build:
	mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(APP) ./cmd/rebuildable

release:
	VERSION=$(VERSION) ./scripts/release.sh

test:
	go test ./...

test-integration:
	go test -tags integration ./...

lint:
	go vet ./...

demo:
	@set -eu; \
	workspace=$$(mktemp -d); \
	trap 'rm -rf "$$workspace"' EXIT; \
	cp -R testdata/fixtures/demo-node/. "$$workspace"; \
	git -C "$$workspace" init -q; \
	git -C "$$workspace" config user.email rebuildable-demo@example.invalid; \
	git -C "$$workspace" config user.name Rebuildable-Demo; \
	git -C "$$workspace" add .; \
	git -C "$$workspace" commit -q -m "demo fixture"; \
	git -C "$$workspace" commit --allow-empty -q -m "demo history"; \
	git -C "$$workspace" commit --allow-empty -q -m "demo history 2"; \
	printf '\n--- before ---\n'; \
	go run ./cmd/rebuildable audit "$$workspace"; \
	printf '\n--- generated diff ---\n'; \
	go run ./cmd/rebuildable fix "$$workspace"; \
	go run ./cmd/rebuildable fix "$$workspace" --write; \
	git -C "$$workspace" add .; \
	printf '\n--- after ---\n'; \
	go run ./cmd/rebuildable audit "$$workspace"
