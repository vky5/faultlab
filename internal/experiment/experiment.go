package experiment

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Experiment is a declarative scenario definition loaded from YAML.
type Experiment struct {
	Name     string         `yaml:"name"`
	Cluster  ClusterConfig  `yaml:"cluster"`
	Timeline []TimelineStep `yaml:"timeline"`
}

// ClusterConfig defines the experiment topology.
type ClusterConfig struct {
	ID      string          `yaml:"id,omitempty"`
	Members []ClusterMember `yaml:"members,omitempty"`
}

// ClusterMember defines a concrete node identity and port loaded from YAML.
type ClusterMember struct {
	ID   string `yaml:"id"`
	Port int    `yaml:"port"`
	Host string `yaml:"host,omitempty"`
}

// TimelineStep is a logical experiment time and its declarative action.
type TimelineStep struct {
	At     ExperimentDuration `yaml:"at"`
	Action string             `yaml:"action"`
	Groups [][]string         `yaml:"groups,omitempty"`
	Key    string             `yaml:"key,omitempty"`
	Value  string             `yaml:"value,omitempty"`
	Target string             `yaml:"target,omitempty"`
}

// CommandBatch is the compiled command list for one timeline step.
type CommandBatch struct {
	At       time.Duration
	Commands []string
}

// CompileOptions controls the command translation step.
type CompileOptions struct {
	ClusterID        string
	ControlPlaneHost string
	ControlPlanePort int
}

// ExperimentDuration keeps YAML-friendly duration parsing separate from scheduling.
type ExperimentDuration time.Duration

func (d *ExperimentDuration) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}

	switch value.Tag {
	case "!!str", "":
		parsed, err := time.ParseDuration(strings.TrimSpace(value.Value))
		if err != nil {
			return fmt.Errorf("parse duration %q: %w", value.Value, err)
		}
		*d = ExperimentDuration(parsed)
		return nil
	case "!!int", "!!float":
		seconds, err := strconv.ParseFloat(strings.TrimSpace(value.Value), 64)
		if err != nil {
			return fmt.Errorf("parse duration %q: %w", value.Value, err)
		}
		*d = ExperimentDuration(time.Duration(seconds * float64(time.Second)))
		return nil
	default:
		return fmt.Errorf("unsupported duration format: %s", value.Tag)
	}
}

func (d ExperimentDuration) Duration() time.Duration {
	return time.Duration(d)
}

// LoadFile parses, normalizes, and validates an experiment YAML file.
func LoadFile(path string) (*Experiment, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read experiment file: %w", err)
	}

	var exp Experiment
	if err := yaml.Unmarshal(raw, &exp); err != nil {
		return nil, fmt.Errorf("parse experiment yaml: %w", err)
	}

	exp.Normalize()
	if err := exp.Validate(); err != nil {
		return nil, err
	}

	return &exp, nil
}

// Normalize sorts the timeline by time and preserves the original order for ties.
func (e *Experiment) Normalize() {
	if e == nil {
		return
	}

	sort.SliceStable(e.Timeline, func(i, j int) bool {
		return e.Timeline[i].At.Duration() < e.Timeline[j].At.Duration()
	})
}

// Validate checks the declarative experiment shape before it is compiled.
func (e *Experiment) Validate() error {
	if e == nil {
		return fmt.Errorf("experiment is nil")
	}
	if strings.TrimSpace(e.Name) == "" {
		return fmt.Errorf("experiment name is required")
	}
	if strings.TrimSpace(e.Cluster.ID) == "" {
		return fmt.Errorf("cluster.id is required")
	}

	if len(e.Cluster.Members) > 0 {
		seen := make(map[string]struct{}, len(e.Cluster.Members))
		for i, member := range e.Cluster.Members {
			if strings.TrimSpace(member.ID) == "" {
				return fmt.Errorf("cluster.members[%d].id is required", i)
			}
			if member.Port <= 0 {
				return fmt.Errorf("cluster.members[%d].port must be greater than zero", i)
			}
			if _, ok := seen[member.ID]; ok {
				return fmt.Errorf("cluster.members[%d].id duplicates %q", i, member.ID)
			}
			seen[member.ID] = struct{}{}
		}
	} else {
		return fmt.Errorf("cluster.members is required")
	}
	if len(e.Timeline) == 0 {
		return fmt.Errorf("experiment timeline is empty")
	}

	for i, step := range e.Timeline {
		if step.At.Duration() < 0 {
			return fmt.Errorf("timeline[%d].at must be >= 0", i)
		}
		if strings.TrimSpace(step.Action) == "" {
			return fmt.Errorf("timeline[%d].action is required", i)
		}
		if err := step.Validate(); err != nil {
			return fmt.Errorf("timeline[%d]: %w", i, err)
		}
	}

	return nil
}

// Validate checks a single timeline step.
func (s TimelineStep) Validate() error {
	switch strings.TrimSpace(s.Action) {
	case "start_cluster":
		return nil
	case "partition", "heal_partition":
		if len(s.Groups) < 2 {
			return fmt.Errorf("action %q requires at least two groups", s.Action)
		}
		for i, group := range s.Groups {
			if len(group) == 0 {
				return fmt.Errorf("action %q groups[%d] is empty", s.Action, i)
			}
			for j, nodeID := range group {
				if strings.TrimSpace(nodeID) == "" {
					return fmt.Errorf("action %q groups[%d][%d] node id is empty", s.Action, i, j)
				}
			}
		}
		return nil
	case "write":
		if strings.TrimSpace(s.Key) == "" {
			return fmt.Errorf("action %q requires key", s.Action)
		}
		if strings.TrimSpace(s.Value) == "" {
			return fmt.Errorf("action %q requires value", s.Action)
		}
		if strings.TrimSpace(s.Target) == "" {
			return fmt.Errorf("action %q requires target", s.Action)
		}
		if len(s.Groups) == 0 {
			return fmt.Errorf("action %q requires groups", s.Action)
		}
		return nil
	default:
		return fmt.Errorf("unknown action %q", s.Action)
	}
}
