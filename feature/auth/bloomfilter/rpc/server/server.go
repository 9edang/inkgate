// Package server implements an rpc server for the bloomfilter, registering a bloomfilter and accepting a tcp listener.
package server

import (
	"fmt"
	"net"
	"net/rpc"

	"github.com/9edang/inkgate/context"

	rpc_bf "github.com/9edang/inkgate/feature/auth/bloomfilter/rpc"
)

// New creates an rpc bloomfilter and launches a serving goroutine.
func New(ctx context.Context, cfg rpc_bf.Config) *rpc_bf.Bloomfilter {
	bf := rpc_bf.New(ctx, cfg)

	//nolint:errcheck // background goroutine, nothing to report the error to
	go Serve(ctx, cfg.Port, bf)

	return bf
}

// Serve creates an rpc server, registers a bloomfilter, accepts a tcp listener and closes when catching context done.
func Serve(ctx context.Context, port int, bf *rpc_bf.Bloomfilter) error {
	s := rpc.NewServer()

	if err := s.Register(&bf.BloomfilterRPC); err != nil {
		return err
	}

	//nolint:noctx // no request context is available at this point
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		// #nosec G104 -- closing is best effort, the error is not actionable
		l.Close()
		bf.Close()
	}()

	s.Accept(l)

	return nil
}
