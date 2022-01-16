package router

import (
	"context"
	"fmt"
	"net"
)

type Endpoint interface {
	net.Listener
	Dial(ctx context.Context, network, address string) (net.Conn, error)
}

type endpoint struct {
	*route
}

func (endpoint *endpoint) Accept() (conn net.Conn, err error) {
	if endpoint.route == nil {
		err = fmt.Errorf("endpoint %w", net.ErrClosed)
		return
	}
	var ok bool
	pipe := endpoint.route.pipe
	conn, ok = <-pipe
	if !ok {
		err = fmt.Errorf("endpoint.route %w", net.ErrClosed)
	}
	return
}

func (endpoint *endpoint) Dial(ctx context.Context, network, address string) (conn net.Conn, err error) {
	if endpoint.route == nil {
		err = fmt.Errorf("endpoint %w", net.ErrClosed)
		return
	}
	return endpoint.route.Dial(ctx, network, address)
}

func (endpoint *endpoint) Close() error {
	if endpoint.route == nil {
		return nil
	}
	return endpoint.route.close(endpoint)
}
