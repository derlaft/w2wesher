package p2p

import (
	"context"
	"errors"
	"time"

	"github.com/derlaft/w2wesher/networkstate"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

const w2wesherTopicName = "w2w:announces"

const announceTimeout = time.Second * 16

func (w *worker) initializePubsub(ctx context.Context) error {
	// initialize gossipsub
	ps, err := pubsub.NewGossipSub(ctx, w.host,
		// this is a small trusted network: enable automatic peer exchange
		pubsub.WithPeerExchange(true),
	)
	if err != nil {
		return err
	}
	w.pubsub = ps

	// join announcements
	topic, err := ps.Join(w2wesherTopicName)
	if err != nil {
		return err
	}
	w.topic = topic

	return nil
}

func (w *worker) consumeAnnounces(ctx context.Context) error {

	// subscribe to the topic
	sub, err := w.topic.Subscribe()
	if err != nil {
		log.
			With("err", err).
			Error("failed to subscribe to announcements")
		return err
	}

	for {
		m, err := sub.Next(ctx)
		if err != nil {

			if errors.Is(err, context.Canceled) {
				return nil
			}

			log.
				With("err", err).
				Error("could not consume a message")
			return err
		}

		if m.ReceivedFrom == w.host.ID() {
			continue
		}

		log.
			With("data", string(m.Message.Data)).
			Debug("got announcement")

		var a networkstate.Announce
		err = a.Unmarshal(m.Message.Data)
		if err != nil {
			log.
				With("err", err).
				Error("could not decode the message")
			return err
		}

		// notify live state about the change
		w.state.OnAnnounce(m.ReceivedFrom, a)

		// connect to the new peer in a non-blocking way
		go w.connect(ctx, a.AddrInfo)

	}

}

func (w *worker) periodicAnnounce(ctx context.Context) error {

	// make a first announce
	w.announceLocal(ctx)

	t := time.NewTicker(w.cfg.P2P.AnnounceInterval)
	defer t.Stop()

	// periodically announce it's own state
	for {
		select {
		case <-t.C:
			w.announceLocal(ctx)
			w.updateAddrs()
		case <-ctx.Done():
			return nil
		}
	}

}

func (w *worker) announceLocal(ctx context.Context) {

	log.Debug("announceLocal")

	ctx, cancel := context.WithTimeout(ctx, announceTimeout)
	defer cancel()

	a := networkstate.Announce{
		AddrInfo: peer.AddrInfo{
			ID:    w.host.ID(),
			Addrs: w.host.Addrs(),
		},
		WireguardState: w.wgControl.AnnounceInfo(),
	}

	log.With("announce", a).Debug("going to send announce")

	data, err := a.Marshal()
	if err != nil {
		log.
			With("err", err).
			Error("could not publish keepalive")
		return
	}

	err = w.topic.Publish(ctx, data)
	if err != nil {
		log.
			With("err", err).
			Error("could not publish keepalive")
	}
}
