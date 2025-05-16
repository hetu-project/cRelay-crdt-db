.PHONY: all build test clean install deps

# 默认目标
all: deps build

# 编译
build:
	go build -o bin/cRelay-crdt-db ./cmd/main.go

# 安装到 GOPATH/bin
install: deps
	go build -o $(GOPATH)/bin/cRelay-crdt-db ./cmd/main.go

# 运行所有测试
test: deps
	go test -v ./...

# 运行特定包的测试
test-handlers: deps
	go test -v ./internal/api/handlers/...

# 运行特定测试文件
test-event-handlers: deps
	go test -v ./internal/api/handlers/event_handlers_test.go

# 运行 orbitdb 包的测试
test-orbitdb: deps
	go test -v ./orbitdb/...

# 运行特定的 orbitdb 测试文件
test-orbitdb-adapter: deps
	go test -v ./orbitdb/adapter_test.go

# 清理编译产物
clean:
	rm -rf bin/
	go clean

# 安装和更新依赖
deps:
	go mod download
	go mod tidy 