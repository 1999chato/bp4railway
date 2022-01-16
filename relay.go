package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/dacapoday/server-meta/request"
	"github.com/dacapoday/server-meta/router"
	socket "github.com/dacapoday/server-meta/socketserver"
	"github.com/dacapoday/server-meta/spine"
	"github.com/dacapoday/server-meta/url"
	"github.com/rs/zerolog"
)

type RelayConfig struct {
	Type  string
	Route []*url.URL
}

type Relay struct {
	logger *zerolog.Logger
	assume func(network, address string) (router.Endpoint, error)
	config *RelayConfig
	stop   func() error
}

func BuildRelay(name string, builder interface {
	spine.LoggerBuilder
	spine.EndpointBuilder
}) (spine.Service, error) {
	return &Relay{
		logger: builder.Logger(name),
		assume: builder.Endpoint,
		config: &RelayConfig{
			Type: name,
		},
	}, nil
}

func (agent *Relay) Close() (err error) {
	agent.logger.Info().Msg("Close")
	if agent.stop != nil {
		err = agent.stop()
		if err == nil {
			agent.stop = nil
		}
	}
	return
}

func (agent *Relay) MarshalJSON() ([]byte, error) {
	agent.logger.Info().Msg("MarshalJSON")
	return json.Marshal(agent)
}

func (agent *Relay) UnmarshalJSON(data []byte) (err error) {
	agent.logger.Info().Msg("UnmarshalJSON")
	var config RelayConfig
	if err = json.Unmarshal(data, &config); err != nil {
		return err
	}

	if agent.config.Type != config.Type {
		return spine.ErrServiceType
	}

	var needRestart bool

	if isSameSliceURL(agent.config.Route, config.Route) {
		needRestart = true
	}
	// TODO: add dial/listen/shutdown timeout config

	agent.config = &config

	if needRestart {
		err = agent.start()
	}
	return
}

func (agent *Relay) start() (err error) {
	agent.logger.Info().Msg("start")

	if agent.stop != nil {
		err = agent.stop()
		if err != nil {
			return
		}
	}

	logger := agent.logger
	assume := agent.assume
	config := agent.config

	remoteURL, err := AccessAgent(&http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
				endpoint, err := assume(
					network,
					addr,
				)
				if err != nil {
					return nil, err
				}
				return open(ctx, endpoint, config.Route)
			},
			ForceAttemptHTTP2:     false,
			MaxIdleConns:          1,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: time.Second * 10,
	}, "relay").GetAgentURL()
	if err != nil {
		return
	}

	listen, err := assume(
		remoteURL.Scheme,
		remoteURL.Host,
	)
	if err != nil {
		return
	}

	forward := socket.NewForwardHandler(func(socket socket.Socket) (net.Conn, error) {
		addr := socket.RemoteAddr()
		endpoint, err := assume(
			addr.Network(),
			addr.String(),
		)
		if err != nil {
			return nil, err
		}
		return open(socket, endpoint, config.Route)
	})

	ctx, cancel := context.WithCancel(context.Background())
	server := &socket.Server{
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	agent.stop = func() (err error) {
		logger.Info().Msg("stop")
		err = server.Close()
		if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			err = nil
		}
		if err != nil {
			logger.Err(err).Msg("stop failed")
		}
		cancel()
		return
	}

	go server.Serve(listen, forward)

	return
}

func open(ctx context.Context, endpoint router.Endpoint, route []*url.URL) (conn net.Conn, err error) {
	if len(route) == 0 {
		return nil, errors.New("route is empty")
	}
	//TODO: timeout: endpoint.Timeout
	conn, err = endpoint.Dial(ctx, route[0].Scheme, route[0].Host)
	if err != nil {
		return
	}
	for _, r := range route[1:] {
		err = request.HttpConnect(ctx, conn, r.Host, nil)
	}
	return
}

func isSameSliceURL(a, b []*url.URL) bool {
	if (a == nil) != (b == nil) {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i].String() != b[i].String() {
			return false
		}
	}

	return true
}
