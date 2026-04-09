package engine

import (
	"github.com/vky5/faultlab/internal/controlplane"
)

type Engine struct {
	ControlPlane *controlplane.ControlPlane
}
