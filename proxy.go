package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/dacapoday/server-meta/proxy"
	"github.com/dacapoday/server-meta/request"
	"github.com/dacapoday/server-meta/router"
	"github.com/dacapoday/server-meta/spine"
	"github.com/dacapoday/server-meta/url"
	"github.com/rs/zerolog"
	uuid "github.com/satori/go.uuid"
)

type Proxy struct{}

type HttpProxyConfig struct {
	Type      string
	Agent     *url.URL
	Listen    *url.URL
	BasicAuth string `json:",omitempty"`
}

type HttpProxy struct {
	logger *zerolog.Logger
	assume func(network, address string) (router.Endpoint, error)
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
	config := agent.config

	endpoint, err := agent.assume(
		config.Agent.Scheme,
		config.Agent.Host,
	)
	if err != nil {
		return
	}

	listener, err := net.Listen("tcp", config.Listen.Host)
	if err != nil {
		endpoint.Close()
		return
	}

	dial := func(ctx context.Context, network string, addr string) (conn net.Conn, err error) {
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
	}

	handler := &proxy.HttpProxy{
		Transport: &http.Transport{
			DialContext: dial,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	server := &http.Server{
		Addr:        config.Listen.Host,
		BaseContext: func(net.Listener) context.Context { return ctx },
		Handler:     handler,
	}
	server.RegisterOnShutdown(cancel)

	agent.stop = func() (err error) {
		logger.Info().Msg("stop")
		err = server.Shutdown(context.Background()) // TODO: stop timeout
		if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			err = nil
		}
		if err != nil {
			logger.Error().Err(err).Msg("stop failed")
		}
		endpoint.Close()
		return
	}

	go server.Serve(listener)
	return
}

type SocketProxy struct{}
