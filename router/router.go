package router

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
)

type Router struct {
	routes sync.Map
	closed bool
}

func (router *Router) routeKey(network, address string) string {
	return network + "://" + address
}

func (router *Router) route(network, address string) (*route, error) {
	val, ok := router.routes.Load(router.routeKey(network, address))
	if !ok {
		return nil, fmt.Errorf("route network:%s address:%s not found", network, address)
	}
	route, ok := val.(*route)
	if !ok {
		return nil, fmt.Errorf("route network:%s address:%s isn't route", network, address)
	}
	return route, nil
}

func (router *Router) close(route *route) (err error) {
	route.Router = nil
	close(route.pipe)
	router.routes.Delete(router.routeKey(route.Network(), route.String()))
	return
}

func (router *Router) Close() (err error) {
	if router.closed {
		return
	}
	router.closed = true
	var routes []interface{}
	router.routes.Range(func(key, v interface{}) bool {
		routes = append(routes, key)
		return true
	})
	var errs []error
	for _, key := range routes {
		v, ok := router.routes.Load(key)
		if !ok {
			continue
		}
		route, ok := v.(io.Closer)
		if !ok {
			continue
		}
		routeErr := route.Close()
		if routeErr != nil {
			errs = append(errs, routeErr)
		}
	}
	if len(errs) > 0 {
		err = fmt.Errorf("%v", errs)
	}
	return
}

func (router *Router) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	if router.closed {
		return nil, fmt.Errorf("Router.Dial %w", net.ErrClosed)
	}
	route, err := router.route(network, address)
	if err != nil {
		return nil, err
	}
	return route.Dial(ctx, network, address)
}

func (router *Router) Listen(ctx context.Context, network, address string) (Endpoint, error) {
	if router.closed {
		return nil, fmt.Errorf("Router.Listen %w", net.ErrClosed)
	}
	key := router.routeKey(network, address)
	val, ok := router.routes.Load(key)
	if !ok {
		val, _ = router.routes.LoadOrStore(key, &route{
			Router:  router,
			network: network,
			address: address,
			pipe:    make(chan net.Conn),
		})
	}
	route, ok := val.(*route)
	if !ok {
		return nil, fmt.Errorf("Router.Listen network:%s address:%s isn't route", network, address)
	}
	return route.Listen(ctx)
}
