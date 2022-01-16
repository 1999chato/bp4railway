package router

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
)

type route struct {
	*Router
	listens int32
	network string
	address string
	pipe    chan net.Conn
}

func (route *route) accept(ctx context.Context, src *route) (conn net.Conn, err error) {
	defer func() {
		if fail := recover(); fail != nil {
			if err, ok := fail.(error); !ok || err.Error() != "send on closed channel" {
				panic(err)
			}
		}
	}()

	if atomic.LoadInt32(&route.listens) < 1 {
		err = fmt.Errorf("route.accept %w", net.ErrClosed)
		return
	}

	conn, dst := Pipe(src.Addr(), route.Addr())
	select {
	case <-ctx.Done():
		err = fmt.Errorf("route.accept %w", ctx.Err())
	case route.pipe <- dst:
	}
	return
}

func (route *route) close(e *endpoint) (err error) {
	e.route = nil
	current := atomic.AddInt32(&route.listens, -1)
	if current == 0 {
		if route.Router != nil {
			route.Router.close(route)
		}
	} else if current < 1 {
		atomic.AddInt32(&route.listens, 1)
	}
	return
}

func (route *route) Dial(ctx context.Context, network, address string) (conn net.Conn, err error) {
	if route.Router == nil {
		err = fmt.Errorf("route.Dial %w", net.ErrClosed)
		return
	}

	dst, err := route.Router.route(network, address)
	if err != nil {
		return
	}

	return dst.accept(ctx, route)
}

func (route *route) Listen(ctx context.Context) (Endpoint, error) {
	if route.Router == nil {
		return nil, fmt.Errorf("route.Listen %w", net.ErrClosed)
	}
	atomic.AddInt32(&route.listens, 1)
	return &endpoint{route: route}, nil
}

func (route *route) Close() error {
	if route.Router == nil {
		return nil
	}
	return route.Router.close(route)
}

func (route *route) Addr() net.Addr { return route }

func (route *route) Network() string { return route.network }

func (route *route) String() string { return route.address }
