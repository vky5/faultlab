package protocol

type ProtocolID string

type MessageKind int

const (
	KindProtocol MessageKind = iota
	KindControl
	KindData
)

// this file defines universal message format used by ALL protocols
type Envelope struct {
	From     string
	To       string
	Protocol ProtocolID
	Payload  []byte

	Kind MessageKind

	Version     uint16
	LogicalTick uint64
}

// for each message we wrap it in an envelope, the message is sent in gRPC
