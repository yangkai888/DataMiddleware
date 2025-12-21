# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Main package
MAIN_PACKAGE=./cmd/server

# Binary name
BINARY_NAME=datamiddleware
BINARY_UNIX=$(BINARY_NAME)_unix

# Build the project
build:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PACKAGE)

# Build for Linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v $(MAIN_PACKAGE)

# Test
test:
	$(GOTEST) -v ./...

# Test coverage
test-coverage:
	$(GOTEST) -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Performance tests
perf-test-tcp:
	$(GOCMD) run test/tcp_performance_test.go

perf-test-async:
	$(GOCMD) run test/async_test.go

perf-test-simple:
	$(GOCMD) run test/tcp_test.go

perf-test-full:
	$(GOCMD) run test/performance_test.go

# Demo programs
demo-async:
	$(GOCMD) run test/async_demo.go

demo-benchmark:
	$(GOCMD) run test/benchmark_demo.go

demo-goroutine-pool:
	$(GOCMD) run test/goroutine_pool_demo.go

demo-memory:
	$(GOCMD) run test/memory_demo.go

demo-performance:
	$(GOCMD) run test/performance_benchmark.go

demo-simple-perf:
	$(GOCMD) run test/simple_perf.go

demo-tcp-perf:
	$(GOCMD) run test/tcp_perf.go

demo-tcp-simple:
	$(GOCMD) run test/tcp_simple.go

# Run all demos
demos: demo-async demo-benchmark demo-goroutine-pool demo-memory demo-performance demo-simple-perf demo-tcp-perf demo-tcp-simple

# Run all performance tests and demos
perf-tests: perf-test-tcp perf-test-async perf-test-simple perf-test-full
all-tests: test perf-tests demos

# Clean
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
	rm -f coverage.out coverage.html

# Run
run:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_PACKAGE)
	./$(BINARY_NAME)

# Dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Format code
fmt:
	$(GOCMD) fmt ./...

# Lint
lint:
	golangci-lint run

# Install development tools
install-tools:
	$(GOGET) -u github.com/golangci/golangci-lint/cmd/golangci-lint
	$(GOGET) -u github.com/cosmtrek/air

# Development with hot reload (requires air)
dev:
	air

.PHONY: build build-linux test test-coverage clean run deps fmt lint install-tools dev
