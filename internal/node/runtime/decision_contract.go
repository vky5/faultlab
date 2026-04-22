package runtime

import "github.com/vky5/faultlab/internal/node/exec"

// Runtime exposes the exec decision contract by delegating to the fault engine.
func (r *Runtime) BeforeSend(peer string) exec.SendDecision {
	if r.fault == nil {
		return exec.SendDecision{Allow: true}
	}
	return r.fault.BeforeSend(peer)
}

func (r *Runtime) BeforeProbe(peer string) exec.ProbeDecision {
	if r.fault == nil {
		return exec.ProbeDecision{Allow: true, Reason: "no-fault-engine"}
	}
	return r.fault.BeforeProbe(peer)
}

func (r *Runtime) BeforeTick() exec.TickDecision {
	if r.fault == nil {
		return exec.TickDecision{Allow: true, Reason: "no-fault-engine"}
	}
	return r.fault.BeforeTick()
}

func (r *Runtime) BeforeReceive(sender string) exec.ReceiveDecision {
	if r.fault == nil {
		return exec.ReceiveDecision{Allow: true}
	}
	return r.fault.BeforeReceive(sender)
}
