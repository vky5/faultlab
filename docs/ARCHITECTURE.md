# FaultLab Architecture Documentation

## Project Overview

**FaultLab** is a distributed systems testing and fault injection framework for:
- **Cluster Management**: Create and organize multiple clusters with pluggable protocols
- **Node Management**: Register, monitor, and control distributed nodes via a central control plane
- **Protocol Testing**: Support for consensus/communication protocol implementations (baseline protocol included)
- **Fault Injection**: Monitor node health, detect failures, and manage cluster state
- **Web Interface**: Next.js dashboard for cluster visualization and management

---

## Logical Architecture Layers

FaultLab is organized into **4 distinct logical layers**:

```
┌─────────────────────────────────────────────────────┐
│ Layer 4: Presentation & Integration                │
│ (CLI, REST API, Web Frontend)                      │
└────────────────┬────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────┐
│ Layer 3: Orchestration & Coordination               │
│ (Control Plane: Actor, Service, Manager)            │
└────────────────┬────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────┐
│ Layer 2: Protocol & Communication                   │
│ (Runtime, Protocol Driver, Protocol Abstraction)    │
└────────────────┬────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────┐
│ Layer 1: Transport & Infrastructure                 │
│ (gRPC, Sessions, Envelope Format)                  │
└─────────────────────────────────────────────────────┘
```

### Layer 1: Transport & Infrastructure
**Location**: `internal/protocol/`, `internal/node/session/`, `internal/transport/`

**Responsibility**: Low-level message transport and session management

**Components**:
- **gRPC Transport**: OrchestratorService (ControlPlane) and NodeService (Node-to-Node)
- **Session Management**: ControlPlaneSession and NodeSession for connection pooling and health tracking
- **Envelope Format**: Protocol-agnostic message wrapper with metadata

**Key Types**:
```go
Envelope {
  From, To: string           // Node identifiers
  Protocol: ProtocolID       // Which protocol this message belongs to
  Payload: []byte            // Serialized protocol-specific data
  Kind: MessageKind          // Protocol/Control/Data
  LogicalTick: uint64        // Protocol logical clock
}
```

---

### Layer 2: Protocol & Communication
**Location**: `internal/node/`, `internal/node/runtime/`, `internal/node/protocol/`

**Responsibility**: Protocol execution, message processing, and distributed algorithm coordination

**Components**:
1. **Protocol Abstraction** - Pluggable interface for consensus algorithms
2. **Protocol Driver** - Manages protocol lifecycle and message routing
3. **Runtime** - Orchestrates protocol driver, event handling, and session initialization
4. **Baseline Protocol** - Reference implementation using gossip-based membership detection

**Key Types**:
```go
ClusterProtocol interface {
  Start(nodeID string) error
  Tick() []Envelope                    // Logical clock advancement
  OnMessage(env Envelope) []Envelope   // Message processing
  Stop() error
  State() any
}

RuntimeEvent = EventTick | EventMessage
```

---

### Layer 3: Orchestration & Coordination
**Location**: `internal/controlplane/`, `internal/cluster/`

**Responsibility**: Centralized cluster state management and node coordination

**Components**:
1. **Actor** - Asynchronous command processor (prevents race conditions)
2. **Service** - Business logic layer for node operations
3. **Manager** - Thread-safe cluster state storage and queries
4. **Heartbeat Manager** - Failure detection and node cleanup

**Key Operations**:
- Node registration with verification
- Heartbeat collection and timeout-based dead node removal
- Peer discovery and list management
- Command sequencing via actor pattern

---

### Layer 4: Presentation & Integration
**Location**: `cmd/controlplane/`, `cmd/node/`, `frontend/`, `internal/controlplane/rest/`

**Responsibility**: External interfaces and user interaction

**Components**:
1. **CLI** - Command-line interface for cluster operations
2. **REST API** - HTTP endpoint for programmatic control (port 8080)
3. **Web Frontend** - Next.js dashboard for visualization

**Supported Commands**:
```
new-cluster <name> <protocol>
add-node <cluster-id> <node-id> <node-address>:<port>
remove-node <cluster-id> <node-id>
list-nodes <cluster-id>
list-clusters
```

---

## System Architecture & Component Interaction

### System Overview Diagram

```mermaid
graph TB
    User[("👤 User")]
    CLI["CLI Interface"]
    REST["REST API (port 8080)"]
    Frontend["Next.js Frontend"]
    
    subgraph ControlPlane ["ControlPlane Process (port 9000)"]
        Actor["Actor<br/>(Command Queue)"]
        Service["Service Layer<br/>(Business Logic)"]
        Manager["Manager<br/>(State Storage)"]
        RPC["gRPC Server<br/>(OrchestratorService)"]
        Cleanup["Cleanup Goroutine<br/>(Heartbeat Timeout)"]
    end
    
    subgraph Node1 ["Node Process 1 (port 7001)"]
        Runtime1["Runtime"]
        Driver1["ProtocolDriver"]
        Protocol1["BaselineProtocol"]
        GRPCServer1["gRPC Server<br/>(NodeService)"]
        Session1["Node Sessions<br/>(RPC Clients)"]
    end
    
    subgraph Node2 ["Node Process 2 (port 7002)"]
        Runtime2["Runtime"]
        Driver2["ProtocolDriver"]
        Protocol2["BaselineProtocol"]
        GRPCServer2["gRPC Server<br/>(NodeService)"]
        Session2["Node Sessions<br/>(RPC Clients)"]
    end
    
    CPSession1["ControlPlane<br/>Session 1"]
    CPSession2["ControlPlane<br/>Session 2"]
    
    User -->|commands| CLI
    User -->|HTTP requests| REST
    User -->|browser| Frontend
    
    CLI -->|parse & submit| Actor
    REST -->|parse & submit| Actor
    Frontend -->|REST calls| REST
    
    Actor -->|execute| Service
    Service -->|read/write| Manager
    Service -->|RPC calls| Session1
    Service -->|RPC calls| Session2
    
    RPC -->|receives| Service
    Manager -->|monitors| Cleanup
    
    CPSession1 -->|RegisterNode<br/>Heartbeat| RPC
    CPSession2 -->|RegisterNode<br/>Heartbeat| RPC
    
    Runtime1 -->|drives| Driver1
    Driver1 -->|tick events| Protocol1
    Runtime1 -->|initializes| Session1
    Session1 -->|peer envelopes| GRPCServer1
    
    Runtime2 -->|drives| Driver2
    Driver2 -->|tick events| Protocol2
    Runtime2 -->|initializes| Session2
    Session2 -->|peer envelopes| GRPCServer2
    
    GRPCServer1 -.->|SendEnvelope| Session2
    GRPCServer2 -.->|SendEnvelope| Session1
    
    Session1 -->|Handshake| GRPCServer2
    Session2 -->|Handshake| GRPCServer1
    
    style ControlPlane fill:#e1f5ff
    style Node1 fill:#f3e5f5
    style Node2 fill:#f3e5f5
    style User fill:#fff9c4
```

---

## Data Flow: Node Registration & Heartbeat Lifecycle

### Complete Node Registration Flow

```mermaid
sequenceDiagram
    participant User
    participant CLIREST as CLI/REST API
    participant CPActor as Actor
    participant Service as Service Layer
    participant Manager as Manager
    participant NodeClient as Node Client (RPC)
    participant Node as Node Runtime
    participant Protocol as Protocol Driver
    participant NodeGRPC as Node gRPC Server
    
    User->>CLIREST: add-node [cluster] [node-id] [host:port]
    CLIREST->>CPActor: Submit RegisterNodeCmd
    CPActor->>Service: Execute RegisterNodeCmd
    Service->>Service: Verify node reachability (ping)
    Service->>Manager: Update cluster state
    Manager-->>Service: OK
    Service->>NodeClient: Send command to node
    NodeClient-->>Service: Acknowledge
    Service-->>CPActor: Success
    CPActor-->>CLIREST: Response
    CLIREST-->>User: Node registered
    
    Note over Node: Meanwhile, node startup...
    
    Node->>Protocol: Initialize BaselineProtocol
    Protocol->>Protocol: Sync membership list
    loop Every tick (1s interval)
        Protocol->>Protocol: Tick()
        alt Tick 5n: Send registration/heartbeat
            Protocol-->>Node: Envelope(RegisterNode)
            Node->>NodeGRPC: Send to ControlPlane
            NodeGRPC->>NodeGRPC: Extract payload
            NodeGRPC->>Service: Route to Service
            Service->>Manager: Update LastSeen timestamp
            Manager-->>Service: OK
            Service-->>NodeGRPC: Response
            NodeGRPC-->>Node: Acknowledge
        end
    end
```

### Failure Detection & Dead Node Removal

```mermaid
sequenceDiagram
    participant Node as Node heartbeat
    participant Protocol as Protocol/Session
    participant ControlPlane as ControlPlane
    participant Manager as State Manager
    participant Cleanup as Cleanup Goroutine
    
    loop Every HEARTBEAT_INTERVAL (5 ticks)
        Node->>Protocol: Send heartbeat envelope
        Protocol->>ControlPlane: gRPC Heartbeat()
        ControlPlane->>Manager: Update LastSeen = now
        Manager-->>ControlPlane: OK
        ControlPlane-->>Protocol: Response ✓
        Protocol-->>Node: Confirmed
    end
    
    Note over Node,Cleanup: No response for threshold ticks...
    
    Node->>Node: Mark connection suspect
    Node->>Node: Increase probe frequency
    
    par Cleanup Loop (5s, background)
        Cleanup->>Manager: Query all nodes
        Manager-->>Cleanup: Node info with LastSeen
        Cleanup->>Cleanup: Check: (now - LastSeen) > TIMEOUT?
        alt Node timed out
            Cleanup->>Manager: Remove node
            Manager-->>Cleanup: OK, removed
            Cleanup->>Cleanup: Log: Node failed
        end
    end
```

---

## Layer Interaction & Communication Patterns

### Communication Matrix

```mermaid
graph LR
    subgraph Presentation["📊 Presentation Layer"]
        CLI["CLI"]
        REST["REST API"]
        UI["Web UI"]
    end
    
    subgraph Orchestration["🎯 Orchestration Layer"]
        Actor["Actor"]
        Service["Service"]
        Manager["Manager"]
    end
    
    subgraph Protocol["🔄 Protocol Layer"]
        Runtime["Runtime"]
        Driver["Protocol Driver"]
        Impl["Protocol Impl"]
    end
    
    subgraph Transport["📡 Transport Layer"]
        CP_RPC["OrchestratorService<br/>gRPC"]
        Node_RPC["NodeService<br/>gRPC"]
        Sessions["Sessions"]
    end
    
    CLI -->|command| Actor
    REST -->|command| Actor
    UI -->|HTTP| REST
    
    Actor -->|call| Service
    Service -->|state ops| Manager
    Service -->|remote calls| CP_RPC
    
    Runtime -->|controls| Driver
    Driver -->|ticks| Impl
    Impl -->|produces| Sessions
    
    Sessions -->|messages| Node_RPC
    Node_RPC -->|receives| Runtime
    
    CP_RPC -->|routes| Service
    
    style CLI fill:#ffccbc
    style REST fill:#ffccbc
    style UI fill:#ffccbc
    style Actor fill:#c8e6c9
    style Service fill:#c8e6c9
    style Manager fill:#c8e6c9
    style Runtime fill:#bbdefb
    style Driver fill:#bbdefb
    style Impl fill:#bbdefb
    style CP_RPC fill:#f8bbd0
    style Node_RPC fill:#f8bbd0
    style Sessions fill:#f8bbd0
```

---

## Component Dependency Graph

### Detailed Dependency Tree

```mermaid
graph TD
    Main["main.go<br/>(ControlPlane)"]
    
    Main --> Config["Config<br/>(flags)"]
    Main --> Manager["Manager<br/>(cluster state)"]
    Main --> Actor["Actor<br/>(command queue)"]
    Main --> RPCServer["RPC Server<br/>(gRPC)"]
    Main --> RESTServer["REST Server<br/>(HTTP)"]
    Main --> Cleanup["Cleanup Goroutine"]
    
    Actor --> Service["Service Layer"]
    Service --> Manager
    Service --> NodeClient["Node Client<br/>(RPC)"]
    
    RPCServer --> Service
    RESTServer --> Service
    Cleanup --> Manager
    
    NodeClient --> Transport["gRPC Transport"]
    Transport --> Proto["Protobuf<br/>(cluster.proto)"]
    
    subgraph Node["Node Components"]
        NodeMain["main.go<br/>(Node)"]
        NodeMain --> NodeConfig["Config<br/>(from flags/file)"]
        NodeMain --> CPSession["ControlPlane<br/>Session"]
        NodeMain --> NodeSession["Node Sessions"]
        NodeMain --> Runtime["Runtime"]
        
        CPSession --> Transport
        NodeSession --> Transport
        
        Runtime --> Driver["Protocol Driver"]
        Driver --> Protocol["Protocol Impl<br/>(BaselineProtocol)"]
        Driver --> Events["Event Channel"]
        Runtime --> GRPCServer["gRPC Server<br/>(NodeService)"]
    end
    
    GRPCServer --> Events
    GRPCServer --> Transport
    
    style Main fill:#c8e6c9
    style Manager fill:#c8e6c9
    style Actor fill:#c8e6c9
    style Service fill:#e1f5fe
    style Driver fill:#f3e5f5
    style Protocol fill:#f3e5f5
    style Transport fill:#fff9c4
```

---

## Protocol Execution Model

### BaselineProtocol Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Initialized: Runtime.Start()
    
    Initialized --> Running: protocol.Start(nodeID)
    
    Running --> Running: Every tick<br/>process ticks<br/>& messages
    
    Running --> Running: Tick counter<br/>incremented
    
    note right of Running
        Tick 5: Send initial RegisterNode
        Tick 5n: Send periodic heartbeats
        Tick 20+: Mark unresponsive peers as Suspect
        Tick 40+: Mark peers as Dead
    end note
    
    Running --> Stopped: protocol.Stop()
    
    Stopped --> [*]: Runtime cleanup
    
    note right of Running
        Timeout handling:
        - 5 tick heartbeat interval
        - 20 tick suspect threshold
        - 40 tick dead threshold
    end note
```

### Message Processing Loop

```mermaid
graph TD
    Start["ProtocolDriver.Run()"]
    Start --> Ticker["Timer tick<br/>1 second"]
    
    Ticker --> Tick1["Call protocol.Tick()"]
    Tick1 --> Env1["⬅️ Returns envelopes<br/>to send"]
    Env1 --> Send1["Send via NodeSession<br/>gRPC"]
    
    Ticker --> Recv["Receive on<br/>event channel"]
    Recv --> Msg["🔹 EventMessage<br/>from peer"]
    Msg --> Process["protocol.OnMessage()"]
    Process --> Env2["⬅️ Returns response<br/>envelopes"]
    Env2 --> Send2["Send responses"]
    
    Send1 --> Ticker
    Send2 --> Ticker
    
    style Start fill:#c8e6c9
    style Ticker fill:#e1f5fe
    style Env1 fill:#bbdefb
    style Env2 fill:#bbdefb
    style Process fill:#f3e5f5
```

---

## Control Plane Command Processing

### Actor-based Command Execution

```mermaid
graph LR
    subgraph Input["Input Sources"]
        CLI["CLI<br/>new-cluster"]
        REST["REST<br/>add-node"]
    end
    
    Input -->|parse command| Parser["Command Parser"]
    Parser -->|create cmd object| Queue["Actor.cmdCh<br/>(buffered channel)"]
    
    Queue -->|dequeue| ActorRun["Actor.Run()<br/>goroutine"]
    
    ActorRun -->|switch on| Execute["Command Executor"]
    
    Execute -->|RegisterNodeCmd| Service1["Service.RegisterNode()"]
    Execute -->|CreateClusterCmd| Service2["Service.CreateCluster()"]
    Execute -->|RemoveNodeCmd| Service3["Service.RemoveNode()"]
    
    Service1 --> Manager["Manager<br/>(state update)"]
    Service2 --> Manager
    Service3 --> Manager
    
    Manager -->|write| State["Cluster State<br/>(RWMutex protected)"]
    
    State -->|reply via| ReplyCh["Actor.replyCh"]
    ReplyCh -->|return| Output["CLI/REST/gRPC<br/>Response"]
    
    style Queue fill:#fff9c4
    style ActorRun fill:#e1f5fe
    style State fill:#c8e6c9
```

---

## Node Startup & Initialization Sequence

```mermaid
sequenceDiagram
    participant Process as Node Process
    participant Config as Config Parser
    participant CPSession as ControlPlaneSession
    participant NodeSession as NodeSession
    participant Runtime as Runtime
    participant Driver as ProtocolDriver
    participant Proto as BaselineProtocol
    participant GRPCServer as gRPC Server
    participant CP as ControlPlane
    
    Process->>Config: Parse flags & INI
    Config-->>Process: NodeConfig
    
    Process->>CPSession: Create (lazy dial)
    Process->>NodeSession: Create (peer connections)
    
    Process->>Runtime: Create with NodeConfig
    
    Process->>Runtime: Start()
    
    Runtime->>Proto: Initialize protocol
    Proto-->>Runtime: Ready
    
    Runtime->>GRPCServer: Start on node port
    GRPCServer-->>Runtime: Listening
    
    Runtime->>Driver: Create ProtocolDriver
    Driver-->>Runtime: Ready
    
    par Event Loop Starts
        Driver->>Driver: Spawn ticker (1s)
        GRPCServer->>GRPCServer: Spawn listener
    end
    
    loop Tick Loop
        Driver->>Proto: Tick()
        Proto-->>Driver: [Envelope]
        
        alt On tick 5
            Driver->>CPSession: Send RegisterNode
            CPSession->>CP: gRPC call
            CP-->>CPSession: Response
        end
    end
```

---

## Protocol Extension Points

### How to Add a New Protocol

The architecture is designed for pluggable protocols. To add a new consensus algorithm:

```mermaid
graph TD
    A["1. Define Protocol Type<br/>implements ClusterProtocol<br/>interface"]
    B["2. Implement Interface"]
    C["3. Register Protocol"]
    D["4. Configure Node<br/>with protocol name"]
    E["5. Runtime Discovery<br/>& Loading"]
    
    A --> B
    B -->|Start<br/>Tick<br/>OnMessage<br/>Stop<br/>State| Registry["Protocol Registry"]
    C --> Registry
    Registry -->|at Node startup| Loader["Protocol Loader"]
    D --> Loader
    Loader -->|create instance| Driver["ProtocolDriver"]
    E --> Driver
    
    style A fill:#fff9c4
    style B fill:#c8e6c9
    style C fill:#e1f5fe
    style Registry fill:#f3e5f5
    style Driver fill:#f3e5f5
```

---

## State Management & Thread Safety

### Manager State Protection

```mermaid
graph TD
    subgraph Manager["Manager (thread-safe)"]
        Lock["RWMutex"]
        Clusters["Clusters map<br/>map[string]*Cluster"]
    end
    
    ReadOp1["Reader 1<br/>GetCluster()"]
    ReadOp2["Reader 2<br/>GetClusters()"]
    WriteOp1["Writer 1<br/>CreateCluster()"]
    WriteOp2["Writer 2<br/>RemoveNode()"]
    
    ReadOp1 -->|RLock| Lock
    ReadOp2 -->|RLock| Lock
    WriteOp1 -->|Lock| Lock
    WriteOp2 -->|Lock| Lock
    
    Lock -->|protects| Clusters
    
    Cleanup["Cleanup Goroutine<br/>removes dead nodes"]
    Cleanup -->|Lock| Lock
    
    style Manager fill:#e1f5fe
    style Lock fill:#fff9c4
    style Cleanup fill:#f3e5f5
```

---

## Information Flow Summary

### Upstream (Node → ControlPlane)

1. **Event Generation**: Protocol driver generates message envelopes on ticks
2. **Envelope Delivery**: Via Node gRPC Session to ControlPlane
3. **Service Routing**: ControlPlane RPC server forwards to Service layer
4. **State Update**: Manager updates cluster state atomically
5. **Cleanup**: Background goroutine monitors for timeouts

### Downstream (ControlPlane → Node)

1. **Command Submission**: CLI/REST submit commands to Actor queue
2. **Execution**: Actor dequeues and delegates to Service
3. **Remote Operations**: Service calls Node via NodeClient (gRPC)
4. **Response**: Node gRPC server processes and replies
5. **Local State**: Node protocol processes responses in OnMessage()

### Peer-to-Peer (Node ↔ Node)

1. **Initiation**: Protocol generates peer envelopes on tick
2. **Handshake**: NodeSession initiates lazy connection with Handshake()
3. **Message Delivery**: SendEnvelope() via gRPC
4. **Reception**: Peer gRPC server receives and queues event
5. **Processing**: ProtocolDriver processes in OnMessage()

---

## Key Architectural Decisions

| Decision | Rationale | Impact |
|----------|-----------|--------|
| **Actor Pattern** | Serializes commands, prevents race conditions | Single point of concurrency control |
| **Protocol Interface** | Allows multiple algorithms on same infrastructure | Extensible, pluggable protocols |
| **Envelope Abstraction** | Protocol-agnostic message wrapper | Node-to-node messages are generic |
| **Lazy Connections** | Reduce resource overhead for sparse clusters | Connection on first message |
| **Heartbeat Timeout** | Distributed failure detection | Eventual consistency of state |
| **Logical Ticks** | Deterministic protocol execution | Reproducible behavior for testing |
| **Two-Tier RPC** | Separation of concerns | OrchestratorService vs NodeService |

---

## System Deployment Model

```mermaid
graph TB
    subgraph Cluster["Cluster (1 per instance)"]
        Node1["Node 1<br/>port 7001"]
        Node2["Node 2<br/>port 7002"]
        NodeN["Node N<br/>port 700N"]
    end
    
    subgraph ControlPlaneInst["ControlPlane Instance"]
        CP["Control Plane<br/>port 9000 (gRPC)<br/>port 8080 (HTTP)"]
    end
    
    subgraph Clients["Clients"]
        CLI["CLI"]
        REST["REST Clients"]
        UI["Browser UI"]
    end
    
    Cluster -->|Register/Heartbeat| ControlPlaneInst
    Cluster -->|Peer messages| Cluster
    
    CLI -->|commands| CP
    REST -->|API calls| CP
    UI -->|HTTP| CP
    
    style Cluster fill:#e1f5fe
    style ControlPlaneInst fill:#f3e5f5
    style CLI fill:#fff9c4
    style REST fill:#fff9c4
    style UI fill:#fff9c4
```

---

## Performance & Scalability Considerations

### Heartbeat Optimization
- Baseline protocol: 5-tick interval (configurable)
- Timeout detection: 20 ticks (configurable)
- Cleanup goroutine: 5-second interval

### Protocol Clock
- Logical ticks: 1-second intervals (per node)
- Prevents clock skew issues
- Enables deterministic replay

### State Access Patterns
- Manager: RWMutex for high read concurrency
- Actor: Buffered channel (default 100 items)
- Sessions: Per-peer connection pooling

---

## Testing & Observability

### Built-in Testing Hooks
- Protocol `State()` method for visibility
- Registration test file: [registration_test.go](../internal/controlplane/rpc/registration_test.go)
- Heartbeat timeout parameters configurable

### Monitoring Points
1. **Node Registration**: Service.RegisterNode() calls
2. **Heartbeat Updates**: Manager.OnHeartbeat() timestamps
3. **Peer Discovery**: GetPeers() responses
4. **Protocol Ticks**: Baseline tick counter
5. **Dead Node Removal**: Cleanup goroutine logs

