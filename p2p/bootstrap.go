package p2p

import (
	"context"
	"time"
)

const rebootstrapInterval = time.Minute * 2

func (w *worker) bootstrapOnce(ctx context.Context) error {

	// initial connect to known peers
	for _, addr := range w.bootstrap {
		go w.connect(ctx, addr)
	}

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
			w.bootstrapOnce(ctx)
		}
	}
}
