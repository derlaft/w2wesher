package p2p

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"golang.org/x/exp/slices"
)

const rebootstrapInterval = time.Second * 10

func (w *worker) initialBootstrap(ctx context.Context) error {
	bootstrap, err := w.cfg.P2P.LoadBootstrapPeers()
	if err != nil {
		return err
	}

	for _, addr := range bootstrap {
		go w.connect(ctx, addr)
	}

	return nil
}

func (w *worker) bootstrapOnce(ctx context.Context) error {

	// list of all known addrs
	var knownAddrs []string

	// extract all peers with addrs
	ps := w.host.Peerstore()
	peersWithAddrs := ps.PeersWithAddrs()

	// generate bootstrap addrs
	for _, p := range peersWithAddrs {
		if p == w.host.ID() {
			// always ignore self
			continue
		}

		ai := ps.PeerInfo(p)

		// force-establish connection with that peer
		go w.connect(ctx, ai)

		// add to the full list of peers
		addrs, _ := peer.AddrInfoToP2pAddrs(&ai)
		for _, addr := range addrs {
			knownAddrs = append(knownAddrs, addr.String())
		}
	}

	// sort && compare
	sort.Strings(knownAddrs)
	var listChanged = !slices.Equal(knownAddrs, w.cfg.P2P.Bootstrap)

	// save config if it is changed
	if listChanged {
		w.cfg.P2P.Bootstrap = knownAddrs
		err := w.cfg.Save()
		if err != nil {
			return fmt.Errorf("bootstrap: failed updating bootstrap peers")
		}
	}

	connectedPeers := w.host.Network().Peers()
	log.Infof("Connected to %v/%v peers", len(connectedPeers), len(peersWithAddrs)-1)

	return nil
}

func (w *worker) periodicBootstrap(ctx context.Context) error {

	t := time.NewTicker(rebootstrapInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			err := w.bootstrapOnce(ctx)
			if err != nil {
				log.
					With("err", err).
					Error("periodic bootstrap failed")

			}
		}
	}
}
