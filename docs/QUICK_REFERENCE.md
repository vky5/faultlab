# FaultLab Developer Quick Reference

Quick lookup guide for developers working with FaultLab.

---

## Project At a Glance

**What it does**: Distributed cluster management and testing framework with pluggable protocol support

**Key Technologies**: Go, gRPC, Protocol Buffers, Next.js, Docker

**Components**: ControlPlane (1), Nodes (N), Frontend (Optional)

**Port allocations**:
- ControlPlane: 9000 (gRPC), 8080 (REST HTTP)
- Nodes: 7001, 7002, 7003, ... (configurable)
- Frontend: 3000 (Next.js dev server)

---

## Directory Structure

```
. (root)
├── cmd/
│   ├── controlplane/main.go       ← Start: go run cmd/controlplane/main.go [flags]
│   └── node/main.go               ← Start: go run cmd/node/main.go [flags]
│
├── internal/
│   ├── cluster/                   ← Cluster & Node state models
│   │   ├── config.go              ← Cluster/Node struct definitions
│   │   └── manager/               ← Thread-safe state manager
│   │
│   ├── controlplane/              ← ControlPlane orchestration
│   │   ├── actor.go               ← Command queue pattern
│   │   ├── command.go             ← Command definitions
│   │   ├── service/service.go     ← Business logic
│   │   ├── rpc/server.go          ← gRPC OrchestratorService
│   │   └── rest/server.go         ← REST API endpoints
│   │
│   ├── node/                      ← Node runtime layer
│   │   ├── config.go              ← Node configuration
│   │   ├── server.go              ← gRPC NodeService
│   │   ├── runtime/
│   │   │   ├── runtime.go         ← Main runtime coordinator
│   │   │   ├── protocol_loop.go   ← Protocol driver (ticks)
│   │   │   └── event.go           ← Event types
│   │   ├── session/
│   │   │   ├── controlplane.go    ← CP communication
│   │   │   └── node.go            ← Node-to-node communication
│   │   ├── protocol/
│   │   │   ├── protocol.go        ← Protocol interface
│   │   │   ├── envelope.go        ← Message wrapper
│   │   │   ├── registry.go        ← Protocol factory
│   │   │   └── baseline/baseline.go ← Example protocol implementation
│   │   └── config/runtime.go      ← Runtime configuration
│   │
│   ├── protocol/                  ← Protocol Buffer definitions
│   │   ├── cluster.proto          ← OrchestratorService
│   │   ├── node.proto             ← NodeService
│   │   └── (generated: *pb.go)   ← Auto-generated protobuf code
│   │
│   └── transport/                 ← Transport abstraction
│       └── grpc_transport.go      ← gRPC implementation
│
├── frontend/                      ← Next.js web dashboard
│   ├── package.json
│   ├── next.config.ts
│   ├── tsconfig.json
│   └── app/                       ← React components
│
├── docs/
│   ├── decisions/                 ← Architecture decisions
│   ├── ARCHITECTURE.md            ← Full architecture guide
│   ├── LAYERS_AND_COMPONENTS.md   ← Layer analysis
│   ├── DATA_FLOWS.md              ← Data flow patterns
│   └── README.md                  ← Getting started
│
├── Makefile                       ← Build commands
├── go.mod                         ← Go dependencies
├── run_test.sh                    ← Test script
└── node.runtime.example.ini       ← Node config template
```

---

## Quick Start Commands

### Run ControlPlane

```bash
# With defaults
go run cmd/controlplane/main.go

# With custom ports
go run cmd/controlplane/main.go \
  --port 9000 \
  --http-port 8080 \
  --heartbeat-timeout 20s
```

### Run Nodes

```bash
# Node 1
go run cmd/node/main.go \
  --id node-1 \
  --port 7001 \
  --cluster-id my-cluster \
  --peers "node-2:7002,node-3:7003" \
  --cp-host localhost \
  --cp-port 9000

# Node 2
go run cmd/node/main.go \
  --id node-2 \
  --port 7002 \
  --cluster-id my-cluster \
  --peers "node-1:7001,node-3:7003" \
  --cp-host localhost \
  --cp-port 9000
```

### Run Frontend

```bash
cd frontend
npm run dev
# Opens at http://localhost:3000
```

### CLI Commands (via ControlPlane stdin)

```bash
# Create cluster
new-cluster my-cluster baseline

# List clusters
list-clusters

# Add node
add-node my-cluster node-1 localhost:7001

# Add more nodes
add-node my-cluster node-2 localhost:7002
add-node my-cluster node-3 localhost:7003

# List nodes in cluster
list-nodes my-cluster

# Remove node
remove-node my-cluster node-1
```

### REST API Calls

```bash
# Create cluster
curl -X POST http://localhost:8080/api/clusters \
  -H "Content-Type: application/json" \
  -d '{"name":"my-cluster","protocol":"baseline"}'

# List clusters
curl http://localhost:8080/api/clusters

# Get cluster details
curl http://localhost:8080/api/clusters/my-cluster

# Add node
curl -X POST http://localhost:8080/api/clusters/my-cluster/nodes \
  -H "Content-Type: application/json" \
  -d '{"nodeId":"node-1","address":"localhost","port":7001}'

# Remove node
curl -X DELETE http://localhost:8080/api/clusters/my-cluster/nodes/node-1
```

---

## Key File Locations for Common Tasks

| Task | Files |
|------|-------|
| Add new gRPC method | `internal/protocol/{cluster,node}.proto` → regenerate |
| Fix CLI command | `internal/controlplane/parser.go` |
| Add REST endpoint | `internal/controlplane/rest/server.go` |
| Implement protocol | `internal/node/protocol/{name}/{name}.go` |
| Debug heartbeat | `internal/cluster/manager/heartbeat_manager.go` |
| Change tick interval | `internal/node/runtime/protocol_loop.go` |
| Update node model | `internal/cluster/config.go` |
| Fix state mgmt | `internal/cluster/manager/manager.go` |

---

## Architecture Layers (Quick View)

```
Layer 4: PRESENTATION
  ├── CLI (stdin commands)
  ├── REST API (:8080)
  └── Web UI (Next.js)
         ↓
Layer 3: ORCHESTRATION (ControlPlane)
  ├── Actor (command queue: serializes access)
  ├── Service (business logic)
  └── Manager (thread-safe state with RWMutex)
         ↓
Layer 2: PROTOCOL & COMMUNICATION (Nodes)
  ├── Runtime (orchestrator)
  ├── ProtocolDriver (tick loop + event dispatch)
  └── Protocol implementation (BaselineProtocol)
         ↓
Layer 1: TRANSPORT & INFRASTRUCTURE
  ├── Sessions (connection pooling)
  ├── gRPC services (OrchestratorService, NodeService)
  └── Protocol Buffers (serialization)
```

---

## Critical Code Paths

### Adding a Node (User → Cluster)

```
CLI/REST input
  → Parser.Parse()
  → Actor.Submit(RegisterNodeCmd)
  → Actor.Run() processes
  → Service.RegisterNode()
    → NodeClient.Ping() [verify reachable]
    → Manager.AddNode()
    → Reply to user
```

**Files involved**: `parser.go` → `service/service.go` → `manager/manager.go`

### Node Registration (Node Process → ControlPlane)

```
Node startup
  → Runtime.Start()
  → ProtocolDriver.Run() (1s ticker)
  → Every 5 ticks, Protocol.Tick() generates RegisterNode envelope
  → NodeSession.Send() to ControlPlane
  → gRPC: SendEnvelope()
  → CP routes to Service.RegisterNode()
  → Manager updates state → LastSeen timestamp
```

**Files involved**: `runtime/runtime.go` → `protocol_loop.go` → `session/controlplane.go` → `service/service.go`

### Heartbeat & Failure Detection

```
Periodic heartbeat from node (every 5 ticks)
  → Service.Heartbeat() updates LastSeen
  
Cleanup goroutine (every 5 seconds)
  → Check if (now - LastSeen) > timeout
  → If yes: Manager.RemoveNode()
  → Node marked as dead/inactive
```

**Files involved**: `heartbeat_manager.go` → `manager.go`

### Peer Discovery

```
Node calls GetPeers from ControlPlane
  → CP Service.GetPeers()
  → Manager returns all nodes in cluster
  → Node updates NodeSession peer list
  → Lazy connections established on first message
```

**Files involved**: `service/service.go` → `session/node.go`

---

## Configuration Reference

### ControlPlane Flags

```
--port VALUE            gRPC server port (default: 9000)
--http-port VALUE       REST API port (default: 8080)
--heartbeat-timeout VALUE  Node timeout (default: 20s)
```

### Node Flags

```
--id VALUE              Node ID (required)
--port VALUE            gRPC server port (required)
--cluster-id VALUE      Cluster ID (required)
--peers VALUE           Comma-separated peer list (e.g., "node2:7002,node3:7003")
--cp-host VALUE         ControlPlane hostname (default: localhost)
--cp-port VALUE         ControlPlane port (default: 9000)
--config VALUE          Path to INI config file
```

### Node INI Config Format (node.runtime.example.ini)

```ini
[node]
id=node-1
port=7001
cluster_id=my-cluster

[peers]
peer_list=node-2:7002,node-3:7003

[controlplane]
host=localhost
port=9000

[protocol]
tick_interval=1s
heartbeat_interval=5
suspect_timeout=20
```

---

## Common Debugging Scenarios

### Problem: Node won't register

**Checks**:
1. Node process running? `ps aux | grep "node/main.go"`
2. ControlPlane reachable? `nc -zv localhost 9000`
3. Check node logs for errors
4. Verify port not in use: `lsof -i :7001`

**Fix**: Restart node with correct flags

### Problem: Heartbeat timeout not working

**Check**:
1. Cleanup goroutine running? Should log every 5s
2. LastSeen timestamp updating? Check Manager state
3. Timeout value too low? Default: 20s
4. Clock skews? Ensure system time synchronized

**File**: `heartbeat_manager.go`

### Problem: Peer communication failing

**Checks**:
1. Peer nodes running? Check ports 7001, 7002, etc.
2. Network connectivity? `ping <peer-host>`
3. Firewall blocking gRPC? 
4. Handshake succeeded? Check logs for handshake errors

**File**: `session/node.go` → `Handshake()` method

### Problem: Protocol not advancing

**Checks**:
1. ProtocolDriver.Run() started? Check error logs
2. Ticker firing? Add `log.Printf("Tick %d", tickCount)` in `protocol_loop.go`
3. Protocol.Tick() returning empty slices? Check BaselineProtocol logic
4. Event channel blocked? Ensure Handler() processes events

**File**: `runtime/protocol_loop.go`

---

## Important Constants & Timeouts

| Feature | Default | Location |
|---------|---------|----------|
| Tick interval | 1s | `protocol_loop.go` |
| Heartbeat interval | 5 ticks | `baseline.go` |
| Suspect timeout | 20 ticks | `baseline.go` |
| Dead timeout | 40 ticks | `baseline.go` |
| Node ping timeout | 2s | `service/service.go` |
| CP heartbeat timeout | 20s | `main.go` flag |
| Cleanup interval | 5s | `heartbeat_manager.go` |
| Actor queue size | 100 | `actor.go` |

---

## Development Workflow

### Making a Change: Adding a New Command

1. **Define command struct**: `internal/controlplane/command.go`
2. **Add parser logic**: `internal/controlplane/parser.go`
3. **Implement service logic**: `internal/controlplane/service/service.go`
4. **Update REST endpoint** (if needed): `internal/controlplane/rest/server.go`
5. **Test**: Write integration test or manual CLI test

### Adding a New Protocol

1. **Create protocol package**: `internal/node/protocol/{name}/{name}.go`
2. **Implement interface**: `ClusterProtocol` (Start, Tick, OnMessage, Stop, State)
3. **Register protocol**: In node `main.go` or auto-register in `init()`
4. **Test**: Create test file with protocol-specific scenarios

### Modifying ControlPlane State

1. **Update model**: `internal/cluster/config.go`
2. **Update manager**: `internal/cluster/manager/manager.go`
3. **Update service logic**: `internal/controlplane/service/service.go`
4. **Thread-safe?**: Check RWMutex usage
5. **Heartbeat affected?**: Check heartbeat manager

### Adding gRPC Method

1. **Define in proto**: `internal/protocol/{cluster,node}.proto`
2. **Generate**: `make proto` or `protoc` command
3. **Implement**: Add method to service implementation
4. **Test**: Write unit test for handler

---

## Build & Test

### Build Commands

```bash
# Build Go binaries
go build -o bin/controlplane cmd/controlplane/main.go
go build -o bin/node cmd/node/main.go

# Or via Makefile (if exists)
make build

# Run tests
go test ./...

# Run specific test
go test ./internal/cluster -v

# Run with coverage
go test -cover ./...
```

### Docker (if applicable)

```bash
# Build docker image
docker build -t faultlab:latest .

# Run container
docker run -p 9000:9000 -p 8080:8080 faultlab:latest
```

---

## Useful grep Patterns

Find specific implementations:

```bash
# Find all protocol implementations
grep -r "type.*Protocol" internal/node/protocol/

# Find heartbeat logic
grep -r "heartbeat" internal/cluster/ --ignore-case

# Find gRPC service definitions
grep -r "service.*Service" internal/protocol/

# Find state mutations
grep -r "Nodes\[" internal/cluster/

# Find RPC client calls
grep -r "NodeClient\." internal/controlplane/

# Find all timeout values
grep -r "timeout\|Timeout" internal/ --ignore-case
```

---

## Performance Notes

- **Actor pattern**: Serializes all commands, handles 100-1000 ops/s
- **Manager RWLock**: Allows concurrent reads, exclusive writes
- **Session pooling**: Reduces connection overhead for sparse clusters
- **Lazy connections**: Peer connections on-demand
- **Logical ticks**: No network clock synchronization needed
- **Cleanup goroutine**: 5s interval suitable for 20+ node clusters

For larger clusters, consider:
- Increasing cleanup interval
- Batching heartbeat responses
- Sharding cluster state

---

## Documentation Reference

- **ARCHITECTURE.md**: Complete architectural overview with Mermaid diagrams
- **LAYERS_AND_COMPONENTS.md**: Layer-by-layer breakdown with visual separation
- **DATA_FLOWS.md**: Detailed data flow patterns and message traces
- **README.md**: Getting started guide
- **docs/decisions/**: Architecture decision records

---

## Useful Debugging Tools

```bash
# Watch GO processes
watch -n 1 'ps aux | grep -E "(controlplane|node)/main"'

# Check listening ports
netstat -tulpn | grep -E ":(7|8|9)[0-9]{3}"
ss -tulpn | grep -E ":(7|8|9)[0-9]{3}"

# Tail logs (if redirected)
tail -f logs/controlplane.log
tail -f logs/node-*.log

# Query REST API with jq
curl -s http://localhost:8080/api/clusters | jq '.clusters | length'

# Monitor gRPC with grpcurl (if installed)
grpcurl -plaintext localhost:9000 list
grpcurl -plaintext localhost:7001 list

# Network debugging
tcpdump -i lo 'tcp port 9000 or tcp port 7001'
```

---

## Common Pitfalls

1. **Node not registering**: Check if ControlPlane listening on correct port
2. **Heartbeat missing**: Verify LastSeen timestamp is being updated
3. **Duplicate nodes in state**: Ensure ID uniqueness across cluster
4. **Race condition on Manager**: Always use RLock/Lock patterns
5. **Actor channel full**: Increase buffer or process faster
6. **Protocol tick frequency too high**: Can overwhelm network
7. **gRPC connection leak**: Ensure sessions properly reused
8. **State inconsistency**: Different views if cleanup too aggressive

---

## Related Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md) - Full system architecture
- [LAYERS_AND_COMPONENTS.md](LAYERS_AND_COMPONENTS.md) - Component analysis
- [DATA_FLOWS.md](DATA_FLOWS.md) - Message and data flows
- [README.md](README.md) - Project overview

