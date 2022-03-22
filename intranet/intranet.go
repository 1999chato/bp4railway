package intranet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
)

type Endpoint interface {
	Dial(ctx context.Context, network, address string) (net.Conn, error)
	net.Listener
}

type Addr struct {
	network string
	address string
}

func (addr Addr) String() string {
	return addr.address
}

func (addr Addr) Network() string {
	return addr.network
}

type Intranet struct {
	Singularity net.Addr
	routes      sync.Map
}

func match(pattern, value string) bool {
	return pattern == value || pattern == ""
}

func (intranet *Intranet) Lookup(network string, pattern ...string) (addrs []net.Addr) {
	intranet.routes.Range(func(key, v interface{}) bool {
		addr, ok := key.(Addr)
		if !ok {
			return true
		}
		if !match(network, addr.network) {
			return true
		}
		if len(pattern) > 0 {
			for _, p := range pattern {
				if match(p, addr.address) {
					addrs = append(addrs, addr)
					continue
				}
			}
		} else {
			addrs = append(addrs, addr)
		}
		return true
	})
	return
}

func (intranet *Intranet) Assume(network, address string) Endpoint {
	return &endpoint{
		intranet: intranet,
		addr: Addr{
			network: network,
			address: address,
		},
	}
}

func (intranet *Intranet) addr() Addr {
	if intranet.Singularity == nil {
		return Addr{"intranet", "intranet"}
	}
	return Addr{intranet.Singularity.Network(), intranet.Singularity.String()}
}

func (intranet *Intranet) Addr() net.Addr {
	return intranet.addr()
}

func (intranet *Intranet) Dial(ctx context.Context, network, address string) (conn net.Conn, err error) {
	singularity := intranet.addr()
	return intranet.
		Assume(singularity.network, singularity.address).
		Dial(ctx, network, address)
}

func (intranet *Intranet) Accept() (conn net.Conn, err error) {
	singularity := intranet.addr()
	value, ok := intranet.routes.Load(singularity)
	if !ok {
		value, _ = intranet.routes.LoadOrStore(singularity, &route{
			pipe:    make(chan net.Conn),
			listens: 1,
		})
	}
	conn, ok = <-value.(*route).pipe
	if !ok {
		err = fmt.Errorf("route %w", net.ErrClosed)
	}
	return
}

func (intranet *Intranet) Close() (err error) {
	routes := intranet.routes
	routes.Range(func(key, value interface{}) bool {
		routes.Delete(key)
		route, ok := value.(*route)
		if ok && atomic.LoadInt32(&route.listens) > 0 {
			route.listens = 0
			close(route.pipe)
		}
		return true
	})
	return
}

func (intranet *Intranet) route(addr Addr) *route {
	value, ok := intranet.routes.Load(addr)
	if ok {
		route, ok := value.(*route)
		if ok {
			return route
		}
	}
	return nil
}

type route struct {
	pipe    chan net.Conn
	listens int32
}

type endpoint struct {
	intranet *Intranet
	*route
	addr Addr
}

func (endpoint *endpoint) Addr() net.Addr { return endpoint.addr }

func (endpoint *endpoint) Dial(ctx context.Context, network, address string) (conn net.Conn, err error) {
	if endpoint.intranet == nil {
		err = fmt.Errorf("endpoint %w", net.ErrClosed)
		return
	}
	intranet := endpoint.intranet
	addr := Addr{network, address}
	route := intranet.route(addr)
	if route == nil {
		route = intranet.route(intranet.addr())
		if route == nil {
			err = &net.OpError{
				Op:     "dial",
				Net:    network,
				Source: endpoint.addr, Addr: addr,
				Err: errors.New("no route"),
			}
			return
		}
	}

	src, dst := Pipe(endpoint.addr, addr)
	select {
	case <-ctx.Done():
		err = &net.OpError{
			Op:     "dial",
			Net:    network,
			Source: endpoint.addr, Addr: addr,
			Err: ctx.Err(),
		}
	case route.pipe <- dst:
		conn = src
	}
	return
}

func (endpoint *endpoint) Accept() (conn net.Conn, err error) {
	if endpoint.intranet == nil {
		err = fmt.Errorf("endpoint %w", net.ErrClosed)
		return
	}
	if endpoint.route == nil {
		intranet := endpoint.intranet
		value, ok := intranet.routes.Load(endpoint.addr)
		if !ok {
			value, _ = intranet.routes.LoadOrStore(endpoint.addr, &route{
				pipe: make(chan net.Conn),
			})
		}
		route, _ := value.(*route)
		atomic.AddInt32(&route.listens, 1)
		endpoint.route = route
	}
	var ok bool
	conn, ok = <-endpoint.route.pipe
	if !ok {
		err = fmt.Errorf("route %w", net.ErrClosed)
	}
	return
}

func (endpoint *endpoint) Close() (err error) {
	if endpoint.intranet != nil {
		if endpoint.route != nil {
			route := endpoint.route
			if atomic.AddInt32(&route.listens, -1) == 0 {
				endpoint.intranet.routes.Delete(endpoint.addr)
				close(route.pipe)
			}
			endpoint.route = nil
		}
		endpoint.intranet = nil
	}
	return
}
