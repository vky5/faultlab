.PHONY: node1 node2 node3 controlplane cluster test-nodes stop

NODE_CMD := go run ./cmd/node
CP_CMD := go run ./cmd/controlplane
CLUSTER_ID := c1

# Start control plane
controlplane:
	$(CP_CMD) -port 9000

# Start nodes (run in separate terminals)
node1:
	$(NODE_CMD) -id node1 -port 7001 --peers node2:7002,node3:7003 -cluster-id $(CLUSTER_ID)

node2:
	$(NODE_CMD) -id node2 -port 7002 --peers node1:7001,node3:7003 -cluster-id $(CLUSTER_ID)

node3:
	$(NODE_CMD) -id node3 -port 7003 --peers node1:7001,node2:7002 -cluster-id $(CLUSTER_ID)

# Start full cluster (controlplane + 3 nodes) in background
cluster:
	@echo "Starting control plane..."
	$(CP_CMD) -port 9000 &
	@sleep 1
	@echo "Starting node1..."
	$(NODE_CMD) -id node1 -port 7001 --peers node2:7002,node3:7003 -cluster-id $(CLUSTER_ID) &
	@sleep 1
	@echo "Starting node2..."
	$(NODE_CMD) -id node2 -port 7002 --peers node1:7001,node3:7003 -cluster-id $(CLUSTER_ID) &
	@sleep 1
	@echo "Starting node3..."
	$(NODE_CMD) -id node3 -port 7003 --peers node1:7001,node2:7002 -cluster-id $(CLUSTER_ID) &
	@echo "Cluster started. Press Ctrl+C to stop all processes."
	@wait

# Stop all cluster processes
stop:
	@echo "Stopping all cluster processes..."
	pkill -f "go run ./cmd/controlplane" || true
	pkill -f "go run ./cmd/node" || true
	@echo "Cluster stopped."

# Test: Start controlplane + nodes, show ping logs
test-nodes:
	@echo "=== Starting Faultlab Cluster Test ==="
	@echo "Terminal 1: Control Plane on :9000"
	@echo "Terminal 2: Node1 on :7001"
	@echo "Terminal 3: Node2 on :7002"
	@echo "Terminal 4: Node3 on :7003"
	@echo ""
	@echo "Run each in separate terminal:"
	@echo "  make controlplane"
	@echo "  make node1"
	@echo "  make node2"
	@echo "  make node3"
	@echo ""
	@echo "Watch for ping logs showing peer health status"
