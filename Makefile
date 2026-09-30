.PHONY: build test vet fmt lint clean demo

# Build both server and client binaries.
build:
	go build -o bin/prready-server ./cmd/prready-server
	go build -o bin/prready-client ./cmd/prready-client
\
# Run all unit tests.
test:
	go test ./...

# Run go vet.
vet:
	go vet ./...

# Check formatting.
fmt:
	@test -z "$$(gofmt -l .)" || (echo "gofmt found unformatted files:" && gofmt -l . && exit 1)

# Run linter (if golangci-lint is installed).
lint:
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || go vet ./...

# Clean build artifacts.
clean:
	rm -rf bin/ reports/

# Run the demo client.
demo: build
	./bin/prready-client --server ./bin/prready-server --config .prready.json

# Run integration tests (requires network).
integration: build
	go test -tags integration ./...

# All quality gates.
check: build vet fmt test
	@echo "All checks passed!"
