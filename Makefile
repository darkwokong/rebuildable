APP := rebuildable

.PHONY: build test test-integration lint demo

build:
	mkdir -p bin
	go build -trimpath -o bin/$(APP) ./cmd/rebuildable

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
	cp -R testdata/fixtures/bad-node/. "$$workspace"; \
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
