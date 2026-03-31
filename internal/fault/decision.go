package fault

import "github.com/vky5/faultlab/internal/node/exec"

// Decision APIs are the external contract consumed by runtime and session.
// This is the single source of truth for fault-order evaluation.
func (e *Engine) BeforeSend(peer string) exec.SendDecision {
	if e == nil {
		return exec.SendDecision{Allow: true, Reason: "no-fault-engine"}
	}

	if e.IsCrashed() {
		return exec.SendDecision{Allow: false, Reason: "crashed"}
	}

	if e.IsPartitioned(peer) {
		return exec.SendDecision{Allow: false, Reason: "partitioned"}
	}

	if e.ShouldDrop() {
		return exec.SendDecision{Allow: false, Reason: "dropped"}
	}

	return exec.SendDecision{Allow: true, Delay: e.GetDelay(), Reason: "allowed"}
}

// BeforeProbe determines if probe traffic to a peer should proceed.
func (e *Engine) BeforeProbe(peer string) exec.ProbeDecision {
	if e == nil {
		return exec.ProbeDecision{Allow: true, Reason: "no-fault-engine"}
	}

	if e.IsCrashed() {
		return exec.ProbeDecision{Allow: false, Reason: "crashed"}
	}

	if e.IsPartitioned(peer) {
		return exec.ProbeDecision{Allow: false, Reason: "partitioned"}
	}

	return exec.ProbeDecision{Allow: true, Delay: e.GetDelay(), Reason: "allowed"}
}

// BeforeTick determines whether protocol/controlplane loops may progress.
func (e *Engine) BeforeTick() exec.TickDecision {
	if e == nil {
		return exec.TickDecision{Allow: true, Reason: "no-fault-engine"}
	}

	if e.IsCrashed() {
		return exec.TickDecision{Allow: false, Reason: "crashed"}
	}

	return exec.TickDecision{Allow: true, Delay: e.GetDelay(), Reason: "allowed"}
}
