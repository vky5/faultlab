package transport

type Message struct {
	Type string
	From string
	Data []byte
}


type Handler func(Message)


type Transport interface {
	Send(peerID string, msg Message) error
	RegisterHandler(h Handler)
}

