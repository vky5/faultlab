package runtime

import "github.com/vky5/faultlab/internal/node/exec"

// Runtime exposes the exec decision contract by delegating to the fault engine.
func (r *Runtime) BeforeSend(peer string) exec.SendDecision {
	if r.fault == nil {
		return exec.SendDecision{Allow: true}
	}
	return r.fault.BeforeSend(peer)
}

func (r *Runtime) BeforeProbe(peer string) bool {
	if r.fault == nil {
		return true
	}
	return r.fault.BeforeProbe(peer)
}

func (r *Runtime) BeforeTick() bool {
	if r.fault == nil {
		return true
	}
	return r.fault.BeforeTick()
}
