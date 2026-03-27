package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ControlPlaneConfig holds controlplane-related configuration.
type ControlPlaneConfig struct {
	// Host is the controlplane host address.
	Host string
	// Port is the controlplane gRPC port.
	Port int
	// GetPeersAfterDelay enables/disables fetching peers after a delay.
	GetPeersAfterDelay bool
	// GetPeersDelay is the delay before fetching peers from controlplane.
	GetPeersDelay time.Duration
	// HeartbeatInterval is the interval between heartbeats.
	HeartbeatInterval time.Duration
	// RegisterOnStart enables/disables automatic registration on startup.
	RegisterOnStart bool
	// RegistrationTimeout is the timeout for registration operation.
	RegistrationTimeout time.Duration
}

// NodeRuntimeConfig holds the complete node runtime configuration.
type NodeRuntimeConfig struct {
	ControlPlane ControlPlaneConfig
}

// DefaultNodeRuntimeConfig returns default runtime configuration.
func DefaultNodeRuntimeConfig() NodeRuntimeConfig {
	return NodeRuntimeConfig{
		ControlPlane: ControlPlaneConfig{
			Host:                "localhost",
			Port:                9000,
			GetPeersAfterDelay:  true,
			GetPeersDelay:       3 * time.Second,
			HeartbeatInterval:   3 * time.Second,
			RegisterOnStart:     true,
			RegistrationTimeout: 5 * time.Second,
		},
	}
}

// LoadNodeRuntimeConfig loads runtime config from an INI-style file.
//
// Format:
//
//	[controlplane]
//	host = localhost
//	port = 9000
//	get_peers_after_delay = true
//	get_peers_delay = 3s
//	heartbeat_interval = 3s
//	register_on_start = true
func LoadNodeRuntimeConfig(path string) (NodeRuntimeConfig, error) {
	cfg := DefaultNodeRuntimeConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("failed to read config file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	currentSection := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}

		// Key-value pair
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])

		if currentSection == "controlplane" {
			switch key {
			case "host":
				cfg.ControlPlane.Host = value
			case "port":
				if p, err := strconv.Atoi(value); err == nil {
					cfg.ControlPlane.Port = p
				}
			case "get_peers_after_delay":
				cfg.ControlPlane.GetPeersAfterDelay, _ = strconv.ParseBool(value)
			case "get_peers_delay":
				if d, err := time.ParseDuration(value); err == nil {
					cfg.ControlPlane.GetPeersDelay = d
				}
			case "heartbeat_interval":
				if d, err := time.ParseDuration(value); err == nil {
					cfg.ControlPlane.HeartbeatInterval = d
				}
			case "register_on_start":
				cfg.ControlPlane.RegisterOnStart, _ = strconv.ParseBool(value)
			case "registration_timeout":
				if d, err := time.ParseDuration(value); err == nil {
					cfg.ControlPlane.RegistrationTimeout = d
				}
			}
		}
	}

	return cfg, nil
}

// SaveNodeRuntimeConfig saves runtime config to an INI-style file.
func SaveNodeRuntimeConfig(path string, cfg NodeRuntimeConfig) error {
	var sb strings.Builder

	sb.WriteString("# Node Runtime Configuration\n\n")

	sb.WriteString("[controlplane]\n")
	sb.WriteString(fmt.Sprintf("host = %s\n", cfg.ControlPlane.Host))
	sb.WriteString(fmt.Sprintf("port = %d\n", cfg.ControlPlane.Port))
	sb.WriteString(fmt.Sprintf("get_peers_after_delay = %v\n", cfg.ControlPlane.GetPeersAfterDelay))
	sb.WriteString(fmt.Sprintf("get_peers_delay = %s\n", cfg.ControlPlane.GetPeersDelay))
	sb.WriteString(fmt.Sprintf("heartbeat_interval = %s\n", cfg.ControlPlane.HeartbeatInterval))
	sb.WriteString(fmt.Sprintf("register_on_start = %v\n", cfg.ControlPlane.RegisterOnStart))
	sb.WriteString(fmt.Sprintf("registration_timeout = %s\n", cfg.ControlPlane.RegistrationTimeout))

	return os.WriteFile(path, []byte(sb.String()), 0644)
}
