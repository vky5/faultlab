package exec

import "time"

type SendDecision struct {
	Allow bool
	Delay time.Duration
}


type FaultDecider interface {
	BeforeSend(peer string) SendDecision
	BeforeProbe(peer string) bool
	BeforeTick() bool
}