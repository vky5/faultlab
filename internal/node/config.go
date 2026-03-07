package node

import (
	"fmt"
	"strconv"
	"strings"
)

// Details related to individual node and its peers
type NodeConfig struct {
	ID    string
	Port  int
	Peers []Peer
}

type Peer struct {
	ID   string
	Host string
	Port int
}

func NewConfig(id string, port int, peersCSV string) (NodeConfig, error) {
	peers, err := parsePeers(peersCSV)
	if err != nil {
		return NodeConfig{}, err
	}

	return NodeConfig{
		ID:    id,
		Port:  port,
		Peers: peers,
	}, nil
}

func parsePeers(raw string) ([]Peer, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	items := strings.Split(raw, ",")
	peers := make([]Peer, 0, len(items))

	for _, item := range items {
		item = strings.TrimSpace(item)

		peerID := ""
		hostPort := item
		if strings.Contains(item, "@") {
			idAndHost := strings.SplitN(item, "@", 2)
			peerID = strings.TrimSpace(idAndHost[0])
			hostPort = strings.TrimSpace(idAndHost[1])
			if peerID == "" {
				return nil, fmt.Errorf("invalid peer %q (missing id before @)", item)
			}
		}

		parts := strings.Split(hostPort, ":")
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("invalid peer %q (expected id:port or id@host:port)", item)
		}

		port, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || port <= 0 {
			return nil, fmt.Errorf("invalid port in peer %q", item)
		}

		host := strings.TrimSpace(parts[0])
		if peerID == "" {
			peerID = host
			host = "localhost"
		}

		peers = append(peers, Peer{
			ID:   peerID,
			Host: host,
			Port: port,
		})
	}

	return peers, nil
}
