.PHONY: all build test clean docker-build

all: build

build:
	# P0-B 后 cmd/ 包不再只有 main.go (新增 db_backup_scheduler.go 等),
	# 单文件构建会报 undefined; 改为按包构建 (测试文件自动排除).
	go build -o bin/platform ./cmd

test:
	go test ./... -v

clean:
	rm -rf bin
	rm -f backend

docker-build:
	docker build -t backend:latest -f Dockerfile .
