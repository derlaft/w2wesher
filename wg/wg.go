package wg

import (
	"context"
	"fmt"
	"hash/fnv"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/derlaft/w2wesher/config"
	"github.com/derlaft/w2wesher/networkstate"
	logging "github.com/ipfs/go-log/v2"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const peerUpdateInterval = time.Minute

var log = logging.Logger("w2wesher:wg")

type Adapter interface {
	Run(context.Context) error
	AnnounceInfo() networkstate.WireguardState
	Update()
}

func (s *State) Run(ctx context.Context) error {

	err := s.InterfaceUp()
	if err != nil {
		return err
	}

	defer func() {
		err = s.InterfaceDown()
		if err != nil {
			log.With("err", err).Fatal("could not cleanup interface")
		}
	}()

	t := time.NewTicker(peerUpdateInterval)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.forceUpdate:
			// force update
			err := s.InterfaceUp()
			if err != nil {
				return err
			}
		case <-t.C:
			// periodic peer update
			err := s.InterfaceUp()
			if err != nil {
				return err
			}
		}
	}
}

// State holds the configured state of a Wesher Wireguard interface.
type State struct {
	iface               string
	client              *wgctrl.Client
	OverlayAddr         netip.Addr
	Port                int
	PrivKey             wgtypes.Key
	PubKey              wgtypes.Key
	overlayPrefix       netip.Prefix
	state               *networkstate.State
	persistentKeepalive *time.Duration
	forceUpdate         chan struct{}
}

// New creates a new Wesher Wireguard state.
// The Wireguard keys are generated for every new interface.
// The interface must later be setup using SetUpInterface.
func New(cfg *config.Config, state *networkstate.State) (Adapter, error) {

	c := cfg.Wireguard

	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("instantiating wireguard client: %w", err)
	}

	privKey, err := wgtypes.ParseKey(c.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("loading private key: %w", err)
	}
	pubKey := privKey.PublicKey()

	prefix, err := netip.ParsePrefix(c.NetworkRange)
	if err != nil {
		return nil, fmt.Errorf("parsing CIDR: %w", err)
	}

	s := State{
		iface:               c.Interface,
		client:              client,
		Port:                c.ListenPort,
		PrivKey:             privKey,
		PubKey:              pubKey,
		state:               state,
		persistentKeepalive: c.PersistentKeepalive,
		forceUpdate:         make(chan struct{}),
		overlayPrefix:       prefix,
	}

	if err := s.assignOverlayAddr(c.NodeName); err != nil {
		return nil, fmt.Errorf("assigning overlay address: %w", err)
	}

	return &s, nil
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

	s.OverlayAddr = addr

	return nil
}

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

func (s *State) AnnounceInfo() networkstate.WireguardState {
	return networkstate.WireguardState{
		PublicKey:    s.PubKey.String(),
		SelectedAddr: s.OverlayAddr.String(),
		Port:         s.Port,
	}
}

func (s State) Update() {
	select {
	case s.forceUpdate <- struct{}{}:
		// force-update sent
	default:
		// Run loop is not waiting right now:
		// it's either not started/stopped
		// or already updating
	}
}
