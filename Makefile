.PHONY: node1 node2 node3 node4 node5 node6 node7 node8 node9 node10 controlplane cpcli cpcli-interactive cluster test-nodes stop config-node1 config-node2 config-node3 config-node4 config-node5 config-node6 config-node7 config-node8 config-node9 config-node10 proto proto-all

NODE_CMD := go run ./cmd/node
CP_CMD := go run ./cmd/controlplane
CPCLI_CMD := go run ./cmd/cpcli
CLUSTER_ID := c1
CONFIG_FILE := node.runtime.ini
Frontend_DIR := ./frontend
PROTO_DIR := internal/protocol

# Build binaries
build: bin/node bin/controlplane

bin/node:
	go build -o bin/node ./cmd/node/main.go

bin/controlplane:
	go build -o bin/controlplane ./cmd/controlplane/main.go

# Start control plane
controlplane:
	$(CP_CMD) -port 9000

# Start cpcli in one-shot mode.
# Usage: make cpcli CMD="list-clusters"
cpcli:
	@if [ -z "$(CMD)" ]; then \
		echo "Usage: make cpcli CMD=\"<controlplane-command>\""; \
		exit 1; \
	fi
	$(CPCLI_CMD) --host localhost --port 9091 $(CMD)

# Start cpcli in interactive mode (continuous input)
cpcli-interactive:
	$(CPCLI_CMD) --interactive --host localhost --port 9091

# Start nodes (run in separate terminals)
node1:
	$(NODE_CMD) -id node1 -port 7001  -cluster-id $(CLUSTER_ID)

node2:
	$(NODE_CMD) -id node2 -port 7002 -cluster-id $(CLUSTER_ID)

node3:
	$(NODE_CMD) -id node3 -port 7003 -cluster-id $(CLUSTER_ID)

node4:
	$(NODE_CMD) -id node4 -port 7004 -cluster-id $(CLUSTER_ID)

node5:
	$(NODE_CMD) -id node5 -port 7005 -cluster-id $(CLUSTER_ID)

node6:
	$(NODE_CMD) -id node6 -port 7006 -cluster-id $(CLUSTER_ID)

node7:
	$(NODE_CMD) -id node7 -port 7007 -cluster-id $(CLUSTER_ID)

node8:
	$(NODE_CMD) -id node8 -port 7008 -cluster-id $(CLUSTER_ID)

node9:
	$(NODE_CMD) -id node9 -port 7009 -cluster-id $(CLUSTER_ID)

node10:
	$(NODE_CMD) -id node10 -port 7010 -cluster-id $(CLUSTER_ID)

# Start nodes with config file (run in separate terminals)
config-node1:
	$(NODE_CMD) -id node1 -port 7001 --peers node2:7002,node3:7003,node4:7004,node5:7005,node6:7006,node7:7007,node8:7008,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) -config $(CONFIG_FILE)

config-node2:
	$(NODE_CMD) -id node2 -port 7002 --peers node1:7001,node3:7003,node4:7004,node5:7005,node6:7006,node7:7007,node8:7008,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) -config $(CONFIG_FILE)

config-node3:
	$(NODE_CMD) -id node3 -port 7003 --peers node1:7001,node2:7002,node4:7004,node5:7005,node6:7006,node7:7007,node8:7008,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) -config $(CONFIG_FILE)

config-node4:
	$(NODE_CMD) -id node4 -port 7004 --peers node1:7001,node2:7002,node3:7003,node5:7005,node6:7006,node7:7007,node8:7008,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) -config $(CONFIG_FILE)

config-node5:
	$(NODE_CMD) -id node5 -port 7005 --peers node1:7001,node2:7002,node3:7003,node4:7004,node6:7006,node7:7007,node8:7008,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) -config $(CONFIG_FILE)

config-node6:
	$(NODE_CMD) -id node6 -port 7006 --peers node1:7001,node2:7002,node3:7003,node4:7004,node5:7005,node7:7007,node8:7008,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) -config $(CONFIG_FILE)

config-node7:
	$(NODE_CMD) -id node7 -port 7007 --peers node1:7001,node2:7002,node3:7003,node4:7004,node5:7005,node6:7006,node8:7008,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) -config $(CONFIG_FILE)

config-node8:
	$(NODE_CMD) -id node8 -port 7008 --peers node1:7001,node2:7002,node3:7003,node4:7004,node5:7005,node6:7006,node7:7007,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) -config $(CONFIG_FILE)

config-node9:
	$(NODE_CMD) -id node9 -port 7009 --peers node1:7001,node2:7002,node3:7003,node4:7004,node5:7005,node6:7006,node7:7007,node8:7008,node10:7010 -cluster-id $(CLUSTER_ID) -config $(CONFIG_FILE)

config-node10:
	$(NODE_CMD) -id node10 -port 7010 --peers node1:7001,node2:7002,node3:7003,node4:7004,node5:7005,node6:7006,node7:7007,node8:7008,node9:7009 -cluster-id $(CLUSTER_ID) -config $(CONFIG_FILE)


# Starting the frontend 
fe:
	cd ${Frontend_DIR} && npm run dev
	


# Start full cluster (controlplane + 10 nodes) in background
cluster:
	@echo "Starting control plane..."
	$(CP_CMD) -port 9000 &
	@sleep 1
	@echo "Starting node1..."
	$(NODE_CMD) -id node1 -port 7001 --peers node2:7002,node3:7003,node4:7004,node5:7005,node6:7006,node7:7007,node8:7008,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) &
	@sleep 1
	@echo "Starting node2..."
	$(NODE_CMD) -id node2 -port 7002 --peers node1:7001,node3:7003,node4:7004,node5:7005,node6:7006,node7:7007,node8:7008,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) &
	@sleep 1
	@echo "Starting node3..."
	$(NODE_CMD) -id node3 -port 7003 --peers node1:7001,node2:7002,node4:7004,node5:7005,node6:7006,node7:7007,node8:7008,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) &
	@sleep 1
	@echo "Starting node4..."
	$(NODE_CMD) -id node4 -port 7004 --peers node1:7001,node2:7002,node3:7003,node5:7005,node6:7006,node7:7007,node8:7008,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) &
	@sleep 1
	@echo "Starting node5..."
	$(NODE_CMD) -id node5 -port 7005 --peers node1:7001,node2:7002,node3:7003,node4:7004,node6:7006,node7:7007,node8:7008,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) &
	@sleep 1
	@echo "Starting node6..."
	$(NODE_CMD) -id node6 -port 7006 --peers node1:7001,node2:7002,node3:7003,node4:7004,node5:7005,node7:7007,node8:7008,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) &
	@sleep 1
	@echo "Starting node7..."
	$(NODE_CMD) -id node7 -port 7007 --peers node1:7001,node2:7002,node3:7003,node4:7004,node5:7005,node6:7006,node8:7008,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) &
	@sleep 1
	@echo "Starting node8..."
	$(NODE_CMD) -id node8 -port 7008 --peers node1:7001,node2:7002,node3:7003,node4:7004,node5:7005,node6:7006,node7:7007,node9:7009,node10:7010 -cluster-id $(CLUSTER_ID) &
	@sleep 1
	@echo "Starting node9..."
	$(NODE_CMD) -id node9 -port 7009 --peers node1:7001,node2:7002,node3:7003,node4:7004,node5:7005,node6:7006,node7:7007,node8:7008,node10:7010 -cluster-id $(CLUSTER_ID) &
	@sleep 1
	@echo "Starting node10..."
	$(NODE_CMD) -id node10 -port 7010 --peers node1:7001,node2:7002,node3:7003,node4:7004,node5:7005,node6:7006,node7:7007,node8:7008,node9:7009 -cluster-id $(CLUSTER_ID) &
	@echo "Cluster started. Press Ctrl+C to stop all processes."
	@wait

# Stop all cluster processes
stop:
	@./scripts/stop.sh

# Test: Start controlplane + nodes, show ping logs
test-nodes:
	@echo "=== Starting Faultlab Cluster Test ==="
	@echo "Terminal 1: Control Plane on :9000"
	@echo "Terminal 2: Node1 on :7001"
	@echo "Terminal 3: Node2 on :7002"
	@echo "Terminal 4: Node3 on :7003"
	@echo "Terminal 5: Node4 on :7004"
	@echo "Terminal 6: Node5 on :7005"
	@echo "Terminal 7: Node6 on :7006"
	@echo "Terminal 8: Node7 on :7007"
	@echo "Terminal 9: Node8 on :7008"
	@echo "Terminal 10: Node9 on :7009"
	@echo "Terminal 11: Node10 on :7010"
	@echo ""
	@echo "Run each in separate terminal:"
	@echo "  make controlplane"
	@echo "  make node1"
	@echo "  make node2"
	@echo "  make node3"
	@echo "  make node4"
	@echo "  make node5"
	@echo "  make node6"
	@echo "  make node7"
	@echo "  make node8"
	@echo "  make node9"
	@echo "  make node10"
	@echo ""
	@echo "With config file:"
	@echo "  make config-node1 CONFIG_FILE=node.runtime.ini"
	@echo ""
	@echo "Watch for ping logs showing peer health status"

# Generate protobuf Go files for one proto by file name.
# Usage:
#   make proto FILE=node
#   make proto FILE=node.proto
proto:
	@if [ -z "$(FILE)" ]; then \
		echo "Usage: make proto FILE=<name|name.proto>"; \
		exit 1; \
	fi
	@command -v protoc >/dev/null 2>&1 || { echo "Error: protoc is not installed"; exit 1; }
	@command -v protoc-gen-go >/dev/null 2>&1 || { echo "Error: protoc-gen-go is not installed"; exit 1; }
	@command -v protoc-gen-go-grpc >/dev/null 2>&1 || { echo "Error: protoc-gen-go-grpc is not installed"; exit 1; }
	@name="$(FILE)"; \
	base="$${name%.proto}"; \
	matches="$$(find "$(PROTO_DIR)" -type f -name "$${base}.proto")"; \
	if [ -z "$$matches" ]; then \
		echo "No proto file found for '$${base}' under $(PROTO_DIR)"; \
		exit 1; \
	fi; \
	for f in $$matches; do \
		echo "Generating $$f"; \
		protoc \
			--proto_path=. \
			--go_out=. --go_opt=paths=source_relative \
			--go-grpc_out=. --go-grpc_opt=paths=source_relative \
			"$$f"; \
	done

# Generate protobuf Go files for all proto files under internal/protocol.
proto-all:
	@command -v protoc >/dev/null 2>&1 || { echo "Error: protoc is not installed"; exit 1; }
	@command -v protoc-gen-go >/dev/null 2>&1 || { echo "Error: protoc-gen-go is not installed"; exit 1; }
	@command -v protoc-gen-go-grpc >/dev/null 2>&1 || { echo "Error: protoc-gen-go-grpc is not installed"; exit 1; }
	@find "$(PROTO_DIR)" -type f -name "*.proto" | while read -r f; do \
		echo "Generating $$f"; \
		protoc \
			--proto_path=. \
			--go_out=. --go_opt=paths=source_relative \
			--go-grpc_out=. --go-grpc_opt=paths=source_relative \
			"$$f"; \
	done
