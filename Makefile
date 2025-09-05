SHELL := /bin/bash

.PHONY: tidy build run test run-agent build-controller build-agent proto proto-tools proto-gen

tidy:
	go mod tidy

build: build-controller build-agent

build-controller:
	go build -o bin/vertera-controller ./cmd/vertera-controller

build-agent:
	go build -o bin/vertera-agent ./cmd/vertera-agent

run:
	go run ./cmd/vertera-controller

run-agent:
	go run ./cmd/vertera-agent

# Run controller with gRPC (mTLS) enabled via build tag
run-controller-mtls:
	VERTERA_GRPC_ADDR=:9090 VERTERA_PKI_DIR=/tmp/vertera/pki \
	go run -tags grpcgen ./cmd/vertera-controller

# Run agent with gRPC (mTLS) enabled via build tag
run-agent-mtls:
	VERTERA_CONTROLLER_ADDR=localhost:9090 VERTERA_PKI_DIR=/tmp/vertera/pki \
	go run -tags grpcgen ./cmd/vertera-agent

test:
	go test ./...

# --- Proto / gRPC codegen scaffolding ---
# We recommend using buf to manage protoc plugins.
# 1) Install tools:
#    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
# 2) Or install buf: https://buf.build/docs/installation
# 3) Place buf.gen.yaml and buf.yaml at repo root, then run `buf generate`.

proto-tools:
	@echo "Ensure protoc plugins installed:" \
	 && echo "  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest" \
	 && echo "  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"

# Generate Go stubs from api/proto/**/*.proto using protoc directly (example):
proto:
	@echo "Generating protobuf stubs..."
	@mkdir -p api/proto
	protoc \
		-I api/proto \
		--go_out=paths=source_relative:./ \
		--go-grpc_out=paths=source_relative:./ \
		api/proto/v1/agent.proto

# If using buf, prefer this target instead of `proto` above.
proto-gen:
	@echo "Run 'buf generate' once buf.yaml is configured."
