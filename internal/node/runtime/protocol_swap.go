package runtime

import (
	"context"
	"fmt"

	proto "github.com/vky5/faultlab/internal/node/protocol"
)

func (n *Runtime) applyProtocolSwap(protocolKey string) error {
	p, err := proto.Load(protocolKey)
	if err != nil {
		return fmt.Errorf("protocol load failed for %s: %w", protocolKey, err)
	}

	if n.proto != nil {
		if err := n.proto.Stop(); err != nil {
			n.logger.Printf("protocol stop warning: %v", err)
		}
	}

	n.proto = p
	n.logger.Printf("Protocol swapped successfully: %T", p)

	peerIDs := make([]string, 0, len(n.config.Peers))
	for _, peer := range n.config.Peers {
		peerIDs = append(peerIDs, peer.ID)
	}

	if peerAwareProtocol, ok := p.(interface{ SetPeers([]string) }); ok {
		peerAwareProtocol.SetPeers(peerIDs)
		n.logger.Printf("Initial peers set for protocol: %v", peerIDs)
	}

	if pWithDiscovery, ok := p.(proto.ClusterProtocolWithDiscovery); ok {
		pWithDiscovery.SetPeerDiscoveryCallback(n)
		n.logger.Printf("Registered peer discovery callback")
	}

	if pWithLogging, ok := p.(proto.ClusterProtocolWithLogging); ok {
		pWithLogging.SetLogger(n.logger)
		n.logger.Printf("Registered remote logger for protocol")
	}

	if err := n.proto.Start(n.config.ID); err != nil {
		return fmt.Errorf("failed to start the new protocol: %w", err)
	}

	// Report capabilities to controlplane after successful swap
	if err := n.reportCapabilitiesToControlPlane(n.ctx, protocolKey, 0); err != nil {
		n.logger.Printf("capability reporting failed after swap: %v", err)
		// Don't fail the swap on reporting error - it's observational
	}

	return nil
}

func (n *Runtime) SendProtocolSwapEvent(
	ctx context.Context,
	protocolKey string,
) error {
	errCh := make(chan error, 1)

	n.eventCh <- RuntimeEvent{
		Type:        EventProtocolSwap,
		ProtocolKey: protocolKey,
		SwapErr:     errCh,
	}

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (n *Runtime) ProtocolSwap(ctx context.Context, protocol string) error {
	return n.SendProtocolSwapEvent(ctx, protocol)
}
