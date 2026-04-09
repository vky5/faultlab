package controlplane

import (
	"context"
	"time"
)

type ControlPlane struct {
	NodeCleanupTimeout time.Duration
	Port               int
	CommandPort        int
	CommandAuthToken   string
	ProjectRoot        string
	DefaultCPHost      string
	DefaultCPPort      int
	NodeBinaryPath     string

	AppCtx context.Context // for closing the control plane gracefully
	cancel context.CancelFunc

	CommandCh <-chan string // raw commands that will be parsed and submitted to actor
}
