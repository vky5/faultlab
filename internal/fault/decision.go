package fault

import "github.com/vky5/faultlab/internal/node/exec"

// Decision APIs are the external contract consumed by components like session.
// They translate current fault parameters into send/probe/tick gate outcomes.
func (e *Engine) BeforeSend(peer string) exec.SendDecision {
	if e == nil {
		return exec.SendDecision{Allow: true}
	}

	if e.IsCrashed() {
		return exec.SendDecision{Allow: false}
	}

	if e.IsPartitioned(peer) {
		return exec.SendDecision{Allow: false}
	}

	if e.ShouldDrop() {
		return exec.SendDecision{Allow: false}
	}

	return exec.SendDecision{Allow: true, Delay: e.GetDelay()}
}

// BeforeProbe determines if a probe to the given peer should succeed based on current fault parameters.
func (e *Engine) BeforeProbe(peer string) bool {
	if e == nil {
		return true
	}

	if e.IsCrashed() {
		return false
	}

	if e.IsPartitioned(peer) {
		return false
	}

	return true
}


// this is to control the tick event, if the node is crashed, we skip
func (e *Engine) BeforeTick() bool {
	if e == nil {
		return true
	}

	return !e.IsCrashed()
}
