package protocol

// PeerDiscoveryCallback defines a callback for dynamic peer discovery.
// Protocol calls this when it discovers a new peer via messages.
type PeerDiscoveryCallback interface {
	OnPeerDiscovered(peerID, peerHost string, peerPort int)
}

// ClusterProtocol interface contract
// envelope defines what message looks like, we now define how to use them

type ClusterProtocol interface {
	Start(nodeID string) error // Called once when runtime starts protocol

	Tick() []Envelope // logical clock advancement

	OnMessage(env Envelope) []Envelope // Handle inbound message

	Stop() error // Stop protocol (cleanup state)

	State() any // for debugging
}

// ClusterProtocolWithDiscovery extends ClusterProtocol with peer discovery callback.
type ClusterProtocolWithDiscovery interface {
	ClusterProtocol
	SetPeerDiscoveryCallback(cb PeerDiscoveryCallback)
}

/*
start() starts basic flow for any algo
Tick is to change the clock for node  Advance logical time, not wall clock. (this is like a timer chain in engine for example after 5 ticks send heartbeat to control plane adn stuff liek that)
OnMessage() is for further processing if some kind of message arrieves
stop is to stop anad state is later for debugging
*/

/*
earlier session was controlling the timing and all for sending the ping to peers
and heartbeat to controlplane. This is to shift that logic here
*/
