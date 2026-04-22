package exec

import "time"

type SendDecision struct {
	Allow  bool
	Delay  time.Duration
	Reason string
}

type ProbeDecision struct {
	Allow  bool
	Delay  time.Duration
	Reason string
}

type ReceiveDecision struct {
	Allow  bool
	Reason string
}

type TickDecision struct {
	Allow  bool
	Delay  time.Duration
	Reason string
}

type FaultDecider interface {
	BeforeSend(peer string) SendDecision
	BeforeReceive(peer string) ReceiveDecision
	BeforeProbe(peer string) ProbeDecision
	BeforeTick() TickDecision
}
