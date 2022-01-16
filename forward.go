package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/dacapoday/server-meta/request"
	"github.com/dacapoday/server-meta/router"
	socket "github.com/dacapoday/server-meta/socketserver"
	"github.com/dacapoday/server-meta/spine"
	"github.com/dacapoday/server-meta/url"
	"github.com/rs/zerolog"
	uuid "github.com/satori/go.uuid"
)

type Forward struct{}

type TcpForwardConfig struct {
	Type    string
	Listen  *url.URL
	Agent   *url.URL
	Forward *url.URL
}

type TcpForward struct {
	logger *zerolog.Logger
	assume func(network, address string) (router.Endpoint, error)
	config *TcpForwardConfig
	stop   func() error
}

func BuildTcpForward(name string, builder interface {
	spine.LoggerBuilder
	spine.EndpointBuilder
}) (spine.Service, error) {
	return &TcpForward{
		logger: builder.Logger(name),
		assume: builder.Endpoint,
		config: &TcpForwardConfig{
			Type: name,
		},
	}, nil
}

func (agent *TcpForward) Close() (err error) {
	agent.logger.Info().Msg("Close")
	if agent.stop != nil {
		err = agent.stop()
		if err == nil {
			agent.stop = nil
		}
	}
	return
}

func (agent *TcpForward) MarshalJSON() ([]byte, error) {
	agent.logger.Info().Msg("MarshalJSON")
	return json.Marshal(agent)
}

func (agent *TcpForward) UnmarshalJSON(data []byte) (err error) {
	agent.logger.Info().Msg("UnmarshalJSON")
	var config TcpForwardConfig
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
	if agent.config.Forward == nil || agent.config.Forward.String() != config.Forward.String() {
		needRestart = true
	}
	// TODO: add dial/listen/shutdown timeout config

	agent.config = &config

	if needRestart {
		err = agent.start()
	}
	return
}

func (agent *TcpForward) start() (err error) {
	agent.logger.Info().Msg("start")

	if agent.stop != nil {
		err = agent.stop()
		if err != nil {
			return
		}
	}

	logger := agent.logger
	config := agent.config

	// TODO: more lite, just a dialer, no need close
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

	forward := socket.NewForwardHandler(func(socket socket.Socket) (conn net.Conn, err error) {
		//TODO: timeout: endpoint.Timeout
		conn, err = endpoint.Dial(socket, config.Agent.Scheme, config.Agent.Host)
		if err != nil {
			return
		}

		traceID := uuid.NewV4().String()
		err = request.HttpConnect(socket, conn, config.Forward.Host,
			http.Header{"X-Trace-Id": []string{traceID}},
		)
		return
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
			logger.Error().Err(err).Msg("stop failed")
		}
		cancel()
		endpoint.Close()
		return
	}

	go server.Serve(listener, forward)

	return
}

type UdpForward struct {
}