package engine

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type RuntimeConfig struct {
	ControlPlane struct {
		Enabled            bool   `yaml:"enabled"`
		Port               int    `yaml:"port"`
		NodeCleanupTimeout string `yaml:"node_cleanup_timeout"`
	} `yaml:"controlplane"`

	Actor struct {
		ProjectRoot    string `yaml:"project_root"`
		DefaultCPHost  string `yaml:"default_cp_host"`
		DefaultCPPort  int    `yaml:"default_cp_port"`
		NodeBinaryPath string `yaml:"node_binary_path"`
	} `yaml:"actor"`

	Nodes []NodeRuntimeConfig `yaml:"nodes"`

	Bootstrap struct {
		Commands []string `yaml:"commands"`
	} `yaml:"bootstrap"`
}

type NodeRuntimeConfig struct {
	ID        string `yaml:"id"`
	Port      int    `yaml:"port"`
	ClusterID string `yaml:"cluster_id"`
	Host      string `yaml:"host"`
	PeersCSV  string `yaml:"peers"`
	CPHost    string `yaml:"cp_host"`
	CPPort    int    `yaml:"cp_port"`
}

func DefaultRuntimeConfig() RuntimeConfig {
	var cfg RuntimeConfig

	cfg.ControlPlane.Enabled = true
	cfg.ControlPlane.Port = 9000
	cfg.ControlPlane.NodeCleanupTimeout = "10s"

	cfg.Actor.ProjectRoot = "."
	cfg.Actor.DefaultCPHost = "localhost"
	cfg.Actor.DefaultCPPort = 9000
	cfg.Actor.NodeBinaryPath = "bin/node"

	return cfg
}

func LoadRuntimeConfig(path string) (RuntimeConfig, error) {
	cfg := DefaultRuntimeConfig()

	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read runtime config: %w", err)
	}

	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parse runtime config yaml: %w", err)
	}

	return cfg, nil
}

func (c RuntimeConfig) ControlPlaneTimeout() (time.Duration, error) {
	if c.ControlPlane.NodeCleanupTimeout == "" {
		return 10 * time.Second, nil
	}

	d, err := time.ParseDuration(c.ControlPlane.NodeCleanupTimeout)
	if err != nil {
		return 0, fmt.Errorf("invalid controlplane.node_cleanup_timeout %q: %w", c.ControlPlane.NodeCleanupTimeout, err)
	}

	return d, nil
}

