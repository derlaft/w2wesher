package wg

import (
	"context"
	"fmt"
	"net/netip"
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
			err := s.UpdatePeers()
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
	// network interface settings
	iface  string
	client *wgctrl.Client
	// wireguard settings
	privKey             wgtypes.Key
	pubKey              wgtypes.Key
	persistentKeepalive *time.Duration
	listenPort          int
	// overlay network address of this node
	overlayAddr netip.Addr
	// overlay network prefix
	overlayPrefix netip.Prefix
	// state of the whole mesh network
	state *networkstate.State
	// peers update channel
	forceUpdate chan struct{}
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
		listenPort:          c.ListenPort,
		privKey:             privKey,
		pubKey:              pubKey,
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

func (s *State) AnnounceInfo() networkstate.WireguardState {
	return networkstate.WireguardState{
		PublicKey:    s.pubKey.String(),
		SelectedAddr: s.overlayAddr.String(),
		Port:         s.listenPort,
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
