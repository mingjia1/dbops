.PHONY: all build test clean docker-build

all: build

build:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/agent-linux-amd64 ./cmd/main.go
	cp bin/agent-linux-amd64 bin/agent
	cp bin/agent-linux-amd64 bin/mysql-ops-agent-linux
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -o bin/agent-windows-amd64.exe ./cmd/main.go

test:
	go test ./... -v

clean:
	rm -rf bin
	rm -f agent

docker-build:
	docker build -t agent:latest -f Dockerfile .
