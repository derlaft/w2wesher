package wg

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"

	"github.com/derlaft/w2wesher/networkstate"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// InterfaceUp creates the interface and routes to the overlay network
func (s *State) InterfaceUp() error {

	log.Debug("InterfaceUp")

	if err := netlink.LinkAdd(&netlink.Wireguard{LinkAttrs: netlink.LinkAttrs{Name: s.iface}}); err != nil && !os.IsExist(err) {
		return fmt.Errorf("creating link %s: %w", s.iface, err)
	}

	link, err := netlink.LinkByName(s.iface)
	if err != nil {
		return fmt.Errorf("getting link information for %s: %w", s.iface, err)
	}

	if err := netlink.AddrReplace(link, &netlink.Addr{
		IPNet: addrToIPNet(s.overlayAddr),
	}); err != nil {
		return fmt.Errorf("setting address for %s: %w", s.iface, err)
	}

	// TODO: make MTU configurable?
	if err := netlink.LinkSetMTU(link, 1420); err != nil {
		return fmt.Errorf("setting MTU for %s: %w", s.iface, err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("enabling interface %s: %w", s.iface, err)
	}

	// add only one route per connection
	if err := netlink.RouteAdd(&netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       prefixToIPNet(s.overlayPrefix),
		Scope:     netlink.SCOPE_LINK,
	}); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("adding route: %w", err)
	}

	return nil
}

// UpdatePeers updates the peers configuration
func (s *State) UpdatePeers() error {

	log.Debug("UpdatePeers")

	nodes := s.state.Snapshot()

	peerCfgs, err := s.peerConfigs(nodes)
	if err != nil {
		return fmt.Errorf("converting received node information to wireguard format: %w", err)
	}

	err = s.client.ConfigureDevice(s.iface, wgtypes.Config{
		PrivateKey: &s.privKey,
		ListenPort: &s.listenPort,
		// even if libp2p connection is broken, we want to keep the old peers
		// to have the best connectivity chances
		ReplacePeers: false,
		Peers:        peerCfgs,
	})
	if err != nil {
		return fmt.Errorf("setting wireguard configuration for %s: %w", s.iface, err)
	}

	return nil
}

// InterfaceDown shuts down the associated network interface.
func (s *State) InterfaceDown() error {
	_, err := s.client.Device(s.iface)
	if err != nil {
		if os.IsNotExist(err) {
			// device already gone; noop
			return nil
		}

		return fmt.Errorf("getting device %s: %w", s.iface, err)
	}

	link, err := netlink.LinkByName(s.iface)
	if err != nil {
		return fmt.Errorf("getting link for %s: %w", s.iface, err)
	}

	return netlink.LinkDel(link)
}

func (s *State) peerConfigs(nodes []networkstate.Info) ([]wgtypes.PeerConfig, error) {
	peerCfgs := make([]wgtypes.PeerConfig, 0, len(nodes))

	for _, node := range nodes {

		as := node.LastAnnounce.WireguardState

		if !as.IsValid() {
			// have not received an announce from that node just yet
			continue
		}

		pubKey, err := wgtypes.ParseKey(as.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("parsing wireguard key: %w", err)
		}

		selectedAddr, err := netip.ParseAddr(as.SelectedAddr)
		if err != nil {
			return nil, fmt.Errorf("parsing selected addr: %w", err)
		}

		peerCfgs = append(peerCfgs, wgtypes.PeerConfig{
			PublicKey:                   pubKey,
			ReplaceAllowedIPs:           true,
			PersistentKeepaliveInterval: s.persistentKeepalive,
			Endpoint: &net.UDPAddr{
				IP:   net.ParseIP(node.Addr),
				Port: s.listenPort,
			},
			AllowedIPs: []net.IPNet{
				*addrToIPNet(selectedAddr),
			},
		})
	}

	return peerCfgs, nil
}
