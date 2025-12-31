.PHONY: proto build run-cluster stop-cluster client clean help install-tools

# Go bin path
GOPATH := $(shell go env GOPATH)
export PATH := $(PATH):$(GOPATH)/bin

# Install protoc Go plugins
install-tools:
	@echo "Installing protoc-gen-go and protoc-gen-go-grpc..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Done! Make sure $(GOPATH)/bin is in your PATH"

# Generate Go code from proto file
proto:
	@echo "Generating protobuf code..."
	PATH="$(PATH):$(GOPATH)/bin" protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/raft.proto

# Build the node and client binaries
build: proto
	@echo "Building node binary..."
	go build -o bin/node.exe ./cmd/node
	@echo "Building client binary..."
	go build -o bin/client.exe ./cmd/client
	@echo "Build complete!"

# Download dependencies
deps:
	go mod tidy
	go mod download

# Run a 5-node cluster
run-cluster: build
	@echo "Starting 5-node RAFT cluster..."
	@mkdir -p logs
	./bin/node.exe -id node1 -address localhost:5001 -peers localhost:5002,localhost:5003,localhost:5004,localhost:5005 > logs/node1.log 2>&1 &
	./bin/node.exe -id node2 -address localhost:5002 -peers localhost:5001,localhost:5003,localhost:5004,localhost:5005 > logs/node2.log 2>&1 &
	./bin/node.exe -id node3 -address localhost:5003 -peers localhost:5001,localhost:5002,localhost:5004,localhost:5005 > logs/node3.log 2>&1 &
	./bin/node.exe -id node4 -address localhost:5004 -peers localhost:5001,localhost:5002,localhost:5003,localhost:5005 > logs/node4.log 2>&1 &
	./bin/node.exe -id node5 -address localhost:5005 -peers localhost:5001,localhost:5002,localhost:5003,localhost:5004 > logs/node5.log 2>&1 &
#	./bin/node.exe -id node6 -address localhost:5006 -peers localhost:5001,localhost:5002,localhost:5003,localhost:5004,localhost:5005 > logs/node6.log 2>&1 &
	@echo "Cluster started! Logs are in ./logs/"
	@echo "Use 'make client' to connect"

# Stop the cluster
stop-cluster:
	@echo "Stopping cluster..."
	-pkill -f "bin/node.exe" 2>/dev/null || true
	@echo "Cluster stopped"

# Run the client
client: build
#	./bin/client.exe -nodes "localhost:5001,localhost:5002,localhost:5003,localhost:5004,localhost:5005,localhost:5006"
	./bin/client.exe -nodes "localhost:5001,localhost:5002,localhost:5003,localhost:5004,localhost:5005"

# View logs
logs:
	@echo "=== Node 1 ===" && tail -20 logs/node1.log 2>/dev/null || echo "No log"
	@echo ""
	@echo "=== Node 2 ===" && tail -20 logs/node2.log 2>/dev/null || echo "No log"
	@echo ""
	@echo "=== Node 3 ===" && tail -20 logs/node3.log 2>/dev/null || echo "No log"
	@echo ""
	@echo "=== Node 4 ===" && tail -20 logs/node4.log 2>/dev/null || echo "No log"
	@echo ""
	@echo "=== Node 5 ===" && tail -20 logs/node5.log 2>/dev/null || echo "No log"

# Follow all logs
follow-logs:
	tail -f logs/*.log

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf logs/
	rm -f proto/*.pb.go

# Run a single node (for testing)
run-node1:
	./bin/node.exe -id node1 -address localhost:5001 -peers localhost:5002,localhost:5003,localhost:5004,localhost:5005

run-node2:
	./bin/node.exe -id node2 -address localhost:5002 -peers localhost:5001,localhost:5003,localhost:5004,localhost:5005

run-node3:
	./bin/node.exe -id node3 -address localhost:5003 -peers localhost:5001,localhost:5002,localhost:5004,localhost:5005

run-node4:
	./bin/node.exe -id node4 -address localhost:5004 -peers localhost:5001,localhost:5002,localhost:5003,localhost:5005

run-node5:
	./bin/node.exe -id node5 -address localhost:5005 -peers localhost:5001,localhost:5002,localhost:5003,localhost:5004

# Help
help:
	@echo "RAFT Consensus Implementation"
	@echo ""
	@echo "Usage:"
	@echo "  make install-tools - Install protoc Go plugins (run once)"
	@echo "  make deps         - Download dependencies"
	@echo "  make proto        - Generate protobuf code"
	@echo "  make build        - Build node and client binaries"
	@echo "  make run-cluster  - Start a 5-node cluster"
	@echo "  make stop-cluster - Stop the cluster"
	@echo "  make client       - Run the client CLI"
	@echo "  make logs         - View node logs"
	@echo "  make follow-logs  - Follow all logs in real-time"
	@echo "  make clean        - Remove build artifacts"
	@echo ""
	@echo "Individual node commands:"
	@echo "  make run-node1    - Run node 1 in foreground"
	@echo "  make run-node2    - Run node 2 in foreground"
	@echo "  make run-node3    - Run node 3 in foreground"
	@echo "  make run-node4    - Run node 4 in foreground"
	@echo "  make run-node5    - Run node 5 in foreground"
