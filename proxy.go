package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"

	m "github.com/dacapoday/marshallable"
	httpserver "github.com/dacapoday/server-meta/http"
	"github.com/dacapoday/server-meta/intranet"
	"github.com/dacapoday/server-meta/proxy"
	"github.com/dacapoday/server-meta/request"
	"github.com/dacapoday/server-meta/spine"
	"github.com/rs/zerolog"
	uuid "github.com/satori/go.uuid"
)

type Proxy struct{}

type HttpProxyConfig struct {
	Type      string
	Agent     *m.URL
	Listen    *m.URL
	BasicAuth string `json:",omitempty"`
}

type HttpProxy struct {
	logger *zerolog.Logger
	assume func(network, address string) (intranet.Endpoint, error)
	config *HttpProxyConfig
	stop   func() error
}

func BuildHttpProxy(name string, builder interface {
	spine.LoggerBuilder
	spine.EndpointBuilder
}) (spine.Service, error) {
	return &HttpProxy{
		logger: builder.Logger(name),
		assume: builder.Endpoint,
		config: &HttpProxyConfig{
			Type: name,
		},
	}, nil
}

func (agent *HttpProxy) Close() (err error) {
	agent.logger.Info().Msg("Close")
	if agent.stop != nil {
		err = agent.stop()
		if err == nil {
			agent.stop = nil
		}
	}
	return
}

func (agent *HttpProxy) MarshalJSON() ([]byte, error) {
	agent.logger.Info().Msg("MarshalJSON")
	return json.Marshal(agent.config)
}

func (agent *HttpProxy) UnmarshalJSON(data []byte) (err error) {
	agent.logger.Info().Msg("UnmarshalJSON")
	var config HttpProxyConfig
	if err = json.Unmarshal(data, &config); err != nil {
		return err
	}

	if agent.config.Type != config.Type {
		return spine.ErrServiceType
	}

	var needRestart bool
	if agent.config.Listen == nil || agent.config.Listen.String() != config.Listen.String() {
		needRestart = true
	}
	if agent.config.Agent == nil || agent.config.Agent.String() != config.Agent.String() {
		needRestart = true
	}
	// TODO: add dial/listen/shutdown timeout config

	agent.config = &config

	if needRestart {
		err = agent.start()
	}
	return
}

func (agent *HttpProxy) start() (err error) {
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

	listener, err := net.Listen("tcp", config.Listen.Host)
	if err != nil {
		return
	}

	// TODO: more lite, just a dialer, no need close
	endpoint, err := assume(
		config.Agent.Scheme,
		config.Agent.Host,
	)
	if err != nil {
		return
	}

	handler := &proxy.HttpProxy{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network string, addr string) (conn net.Conn, err error) {
				//TODO: timeout: endpoint.Timeout
				conn, err = endpoint.Dial(ctx, config.Agent.Scheme, config.Agent.Host)
				if err != nil {
					return
				}

				traceID := uuid.NewV4().String()
				err = request.HttpConnect(ctx, conn, addr,
					http.Header{"X-Trace-Id": []string{traceID}},
				)
				return
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	server := &httpserver.Server{
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	agent.stop = func() (err error) {
		logger.Info().Msg("stop")
		err = server.Shutdown(context.Background()) // TODO: stop timeout
		if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			err = nil
		}
		if err != nil {
			logger.Error().Err(err).Msg("stop failed")
		}
		cancel()
		endpoint.Close()
		return
	}

	go server.Serve(listener, handler)
	return
}

type SocketProxy struct{}
