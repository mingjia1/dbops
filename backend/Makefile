.PHONY: all build test clean docker-build

all: build

build:
	go build -o bin/platform ./cmd/main.go

test:
	go test ./... -v

clean:
	rm -rf bin
	rm -f backend

docker-build:
	docker build -t backend:latest -f Dockerfile .
