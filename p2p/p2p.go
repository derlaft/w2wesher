package p2p

import (
	"context"
	"time"

	"github.com/derlaft/w2wesher/config"
	"github.com/derlaft/w2wesher/networkstate"
	"github.com/derlaft/w2wesher/runnergroup"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/multiformats/go-multiaddr"
	"golang.org/x/sync/semaphore"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

const maxParallelConnects = 8
const connectTimeout = time.Second * 16

var log = logging.Logger("w2wesher:p2p")

type Node interface {
	Run(context.Context) error
}

type Wireguard interface {
	AnnounceInfo() networkstate.WireguardState
	Update()
}

type worker struct {
	host             host.Host
	topic            *pubsub.Topic
	pubsub           *pubsub.PubSub
	pk               crypto.PrivKey
	psk              []byte
	state            *networkstate.State
	wgControl        Wireguard
	newConnectionSem *semaphore.Weighted
	cfg              *config.Config
}

func New(cfg *config.Config, state *networkstate.State, wgControl Wireguard) (Node, error) {

	pk, err := cfg.P2P.LoadPrivateKey()
	if err != nil {
		return nil, err
	}

	psk, err := cfg.P2P.LoadPsk()
	if err != nil {
		return nil, err
	}

	return &worker{
		cfg:              cfg,
		pk:               pk,
		psk:              psk,
		state:            state,
		wgControl:        wgControl,
		newConnectionSem: semaphore.NewWeighted(maxParallelConnects),
	}, nil
}

func (w *worker) connect(ctx context.Context, p peer.AddrInfo) {
	err := w.newConnectionSem.Acquire(ctx, 1)
	if err != nil {
		log.
			With("err", err).
			Error("failed acquire newConnectionSem")
		return
	}
	defer w.newConnectionSem.Release(1)

	// context timeout
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	log.With("addr", p).Debug("connecting to the peer")
	err = w.host.Connect(ctx, p)
	if err != nil {
		log.
			With("addr", p).
			With("err", err).
			Error("failed to connect to the peer")
	}
}

func (w *worker) updateAddrs() {
	ret := make(map[peer.ID]multiaddr.Multiaddr)
	n := w.host.Network()

	for _, peer := range n.Peers() {
		for _, addr := range n.ConnsToPeer(peer) {
			ret[peer] = addr.RemoteMultiaddr()

			// we can only use one addr in wg
			break
		}
	}

	w.state.UpdateAddrs(ret)
	w.wgControl.Update()
}

func (w *worker) Run(ctx context.Context) error {

	log.Debug("starting")

	// make sure it fails on invalid psk
	pnet.ForcePrivateNetwork = true

	h, err := libp2p.New(
		libp2p.Identity(w.pk),
		libp2p.ListenAddrStrings(w.cfg.P2P.ListenAddr),
		libp2p.PrivateNetwork(w.psk),
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
	)
	if err != nil {
		return err
	}
	w.host = h

	err = w.initializePubsub(ctx)
	if err != nil {
		return err
	}

	err = w.initialBootstrap(ctx)
	if err != nil {
		return err
	}

	log.
		With("id", h.ID().String()).
		Info("initialization complete, starting periodic updates")

	return runnergroup.
		New(ctx).
		Go(w.periodicAnnounce).
		Go(w.consumeAnnounces).
		Go(w.periodicBootstrap).
		Go(w.sendWelcomeAnnounces).
		Wait()
}
