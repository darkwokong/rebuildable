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
	go run ./cmd/rebuildable audit testdata/fixtures/good-go
