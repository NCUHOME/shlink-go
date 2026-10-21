BINARY := bin/shlink-go

.PHONY: build run test test-integration test-all fmt vet lint clean

build:
	go build -o $(BINARY) ./cmd/shlink-go

run:
	go run ./cmd/shlink-go

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

test-all: test test-integration

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	go vet ./...

lint:
	golangci-lint run

clean:
	rm -rf bin dist
