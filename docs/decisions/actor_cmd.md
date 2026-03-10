see now I want to make the mamain.go for orchestrator such that it not only starts the server but it also accepts commands and in that command, we cna create a blank cluster like --- new-cluster -stop nodeid -create node id -cluister id like this thjis will be great right? I am thkning abotu actor model in a separate goroutine accepts the incoming command send it to channel and a single process will process it but nah we can run goroutine instead. sounds like a plan?


Plan is good, with one change: keep a **single command processor** (actor-style), not a goroutine per command.

Why:
- Per-command goroutines make ordering unpredictable (`create`, `stop`, `delete` races).
- Single processor gives deterministic state transitions.

Recommended shape in `cmd/controlplane/main.go`:
1. Start gRPC server in one goroutine.
2. Start stdin/CLI reader goroutine (`bufio.Scanner`) that parses commands and sends typed `Command` to a channel.
3. Run one command loop that reads from channel and calls `cluster.Manager`.

Command ideas:
- `new-cluster <cluster_id>`
- `create-node <cluster_id> <node_id> <host> <port>` (or just register metadata)
- `stop-node <cluster_id> <node_id>` (usually means deregister/mark stopped unless you add an RPC to actually stop process)
- `list-clusters`
- `list-nodes <cluster_id>`

Important detail:
- “create node” from orchestrator won’t start a real node process unless you intentionally spawn OS processes. Usually orchestrator only tracks/controls via RPC.

If you want, I can implement this command loop in `cmd/controlplane/main.go` incrementally.


---
btw this is from the chatgpt and I am not planning to create node like this, instead the create node will be run like normally and then the 