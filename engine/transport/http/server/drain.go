package server

import (
	"sync"

	"github.com/9edang/inkgate/context"
)

// Drain tells long-lived responses (SSE streams, WebSocket relays) that the server is shutting down, and lets the
// shutdown wait for them. http.Server.Shutdown neither tells a running handler to stop nor waits for a hijacked
// (WebSocket) connection, so without it open streams hold the shutdown and sockets are cut when the process exits.
type Drain struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewDrain returns a Drain that has not started.
func NewDrain() *Drain {
	ctx, cancel := context.WithCancel(context.Background())

	return &Drain{ctx: ctx, cancel: cancel}
}

// Done is closed when the shutdown starts. A nil Drain is never done.
func (d *Drain) Done() <-chan struct{} {
	if d == nil {
		return nil
	}

	return d.ctx.Done()
}

// Context is cancelled when the shutdown starts. A nil Drain gives a context that is never cancelled.
func (d *Drain) Context() context.Context {
	if d == nil {
		return context.Background()
	}

	return d.ctx
}

// Track registers a long-lived response until the returned function is called; the shutdown waits for it.
func (d *Drain) Track() (done func()) {
	if d == nil {
		return func() {}
	}
	d.wg.Add(1)

	return d.wg.Done
}

// Start signals the shutdown to the tracked responses.
func (d *Drain) Start() { d.cancel() }

// Wait waits for the tracked responses to end, or for ctx.
func (d *Drain) Wait(ctx context.Context) error {
	ended := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(ended)
	}()
	select {
	case <-ended:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type drainKey struct{}

// WithDrain stores d in ctx; the server puts it in the context of every request.
func WithDrain(ctx context.Context, d *Drain) context.Context {
	return context.WithValue(ctx, drainKey{}, d)
}

// DrainFrom returns the Drain of the server handling the request, or nil (every method of a nil Drain is safe).
func DrainFrom(ctx context.Context) *Drain {
	d, _ := ctx.Value(drainKey{}).(*Drain)

	return d
}
