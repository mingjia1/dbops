.PHONY: all build test clean docker-build

all: build

build:
	go build -o bin/platform ./cmd

test:
	go test ./... -v

clean:
	rm -rf bin
	rm -f platform-backend

docker-build:
	docker build -t platform-backend:latest -f Dockerfile.test .