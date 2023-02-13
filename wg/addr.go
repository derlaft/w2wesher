package wg

import (
	"fmt"
	"hash/fnv"
	"net"
	"net/netip"
	"os"
)

func addrToIPNet(addr netip.Addr) *net.IPNet {
	return &net.IPNet{
		IP:   addr.AsSlice(),
		Mask: net.CIDRMask(addr.BitLen(), addr.BitLen()),
	}
}

func prefixToIPNet(p netip.Prefix) *net.IPNet {
	addr := p.Addr()
	return &net.IPNet{
		IP:   addr.AsSlice(),
		Mask: net.CIDRMask(p.Bits(), addr.BitLen()),
	}
}

// assignOverlayAddr assigns a new address to the interface.
// The address is assigned inside the provided network and depends on the
// provided name deterministically.
// Currently, the address is assigned by hashing the name and mapping that
// hash in the target network space.
func (s *State) assignOverlayAddr(nodeName string) error {

	if nodeName == "" {
		nodeName, _ = os.Hostname()
	}

	ip := s.overlayPrefix.Addr().AsSlice()

	h := fnv.New128a()
	h.Write([]byte(nodeName))
	hb := h.Sum(nil)

	for i := 1; i <= (s.overlayPrefix.Addr().BitLen()-s.overlayPrefix.Bits())/8; i++ {
		ip[len(ip)-i] = hb[len(hb)-i]
	}

	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return fmt.Errorf("could not create IP from %q", ip)
	}

	log.With("addr", addr).Debug("assigned overlay address")

	s.overlayAddr = addr

	return nil
}
