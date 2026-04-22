package gossip

type MessageType string

const (
	MsgDigest MessageType = "DIGEST" // Digest → Compare → Push missing data
	MsgState  MessageType = "STATE"
)

type Value struct {
	Data        string           `json:"data"`
	Version     int64            `json:"version"`
	Origin      string           `json:"origin"`
	Timestamp   int64            `json:"timestamp"`
	VectorClock map[string]int64 `json:"vector_clock"`
}

const (
	TimelineEventWrite         = "WRITE"
	TimelineEventGossipReceive = "GOSSIP_RECEIVE"
	TimelineEventResolve       = "RESOLVE"
)

// DigestEntry is the compact metadata summary for one key.
// It intentionally excludes full Data to keep DIGEST lightweight.
type DigestEntry struct {
	Version     int64            `json:"version"`
	Origin      string           `json:"origin"`
	Timestamp   int64            `json:"timestamp"`
	VectorClock map[string]int64 `json:"vector_clock"`
}

// Envelope for gossip messages
type GossipMessage struct {
	Type MessageType `json:"type"`

	Digest map[string]DigestEntry `json:"digest,omitempty"`
	State  map[string]Value       `json:"state,omitempty"`
}

/*
? This is the architecture for PUSH BASESD GOSSIP PROTOCOL “I see what you’re missing → I send it to you”
? Pull - “I see what I’m missing → I ask you for it”
? Push pull
A → B : DIGEST
B → A : DIGEST
A → B : STATE (what B needs)
B → A : STATE (what A needs)



! this is what we are gonna implement, PUSH BASED GOSSIP PROTOCOL for now
* State = actual data + metadata
* Digest = key + compact metadata summary

Digest example:
"a" → {version: 5, origin: "nodeA", timestamp: 1712831204000}
"b" → {version: 2, origin: "nodeB", timestamp: 1712831181000}

I have key a at version 5 written by nodeA at t1, key b at version 2 written by nodeB at t0.

Need of digest?
Sending full data every time is stupid
- Waste bandwidth
- duplicate data
- slow

State example:
"a" → {Data: "hello", Version: 5, Origin: "nodeA", Timestamp: 1712831204000}
"b" → {Data: "world", Version: 2, Origin: "nodeB", Timestamp: 1712831181000}
it contains the actual data and its version, so that the receiver can compare it with its own version and decide whether to update or not.

? LWW order we use for comparison:
* Higher Timestamp wins.
* If Timestamp ties, lexicographically higher Origin wins.
* If Origin ties too, higher Version wins.
* If all tie, values are considered equivalent for merge ordering.


How it works??
A -> B: Digest (a: 5, b: 2)

B compares:
a:3
b:2
c:7

B sends to A state
B → A : STATE {c:7}

A merges C



One more thing,
the current implementation behaves like push-pull on digest exchange.

If B sees A is behind, B sends STATE to A.
If B sees B is behind, B sends its DIGEST back to A so A can answer with STATE.

For example:
A sends DIGEST with a:6
B has a:3 and c:7

B sends STATE {c:7} to A because A is missing c.
B also sends its own DIGEST back because B is behind on a.
A can then reply with STATE for a:6.
*/
