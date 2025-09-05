# Build all
help:
	@echo "Available targets:"
	@echo "  tidy        - go mod tidy"
	@echo "  build       - build controller and agent"
	@echo "  run         - run controller (HTTP only)"
	@echo "  run-agent   - run agent"
	@echo "  run-controller-mtls - run controller with gRPC mTLS (tag grpcgen)"
	@echo "  run-agent-mtls      - run agent with gRPC mTLS (tag grpcgen)"
	@echo "  proto       - generate protobuf stubs (protoc)"
	@echo "  proto-tools - echo instructions to install protoc plugins"
	@echo "  proto-gen   - buf generate placeholder"
	@echo "  lint        - run golangci-lint if installed"
	@echo "  docs        - echo docs URL and curl check"

SHELL := /bin/bash

.PHONY: tidy build run test run-agent build-controller build-agent proto proto-tools proto-gen lint docs help

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
		--go_out=api/proto --go_opt=paths=source_relative \
		--go-grpc_out=api/proto --go-grpc_opt=paths=source_relative \
		api/proto/v1/agent.proto

# If using buf, prefer this target instead of `proto` above.
proto-gen:
	@echo "Run 'buf generate' once buf.yaml is configured."

# Lint (requires golangci-lint in PATH)
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ; \
	else \
		echo "golangci-lint not found. Install: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b \`go env GOPATH\`/bin v2.4.0" ; \
		exit 1 ; \
	fi

# Docs quick check
docs:
	@echo "Docs URL: http://localhost:8080/api/v1/docs/index.html"
	@echo "Spec URL: http://localhost:8080/api/v1/openapi.yaml"
	@echo "Checking spec endpoint..."
	@curl -sI http://localhost:8080/api/v1/openapi.yaml | head -n1 || true
