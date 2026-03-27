package gossip

type MessageType string

const (
	MsgDigest MessageType = "DIGEST" // Digest → Compare → Push missing data
	MsgState MessageType = "STATE"
)


type Value struct {
	Data string
	Version int64
}

// Envelope for gossip messages
type GossipMessage struct {
	Type MessageType `json:"type"`

	Digest map[string]int64 `json:"digest,omitempty"`
	State map[string]Value `json:"state,omitempty"`
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
* State = actual data
* Digest = key and version of the data

Digest example:
"a" → 5
"b" → 2

I have a key a at version 5, key b at version 2

Need of digest?
Sending full data every time is stupid
- Waste bandwidth 
- duplicate data
- slow

State example:
"a" → {Data: "hello", Version: 5}
"b" → {Data: "world", Version: 2}
it contains the actual data and its version, so that the receiver can compare it with its own version and decide whether to update or not.


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
B never queries A for update

for example a: 6 in A
and when A snds the state, B compares a and sees B is behind x 
& A is missing c, so B sends the state to A, and A merges it.
But it wont request the updated a
*/ 