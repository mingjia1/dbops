.PHONY: all build test clean docker-build

all: build

build:
	go build -o bin/agent ./cmd/main.go

test:
	go test ./... -v

clean:
	rm -rf bin
	rm -f agent

docker-build:
	docker build -t agent:latest -f Dockerfile .
