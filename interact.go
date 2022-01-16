package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/dacapoday/server-meta/router"
	socket "github.com/dacapoday/server-meta/socketserver"
	"github.com/dacapoday/server-meta/spine"
	"github.com/dacapoday/server-meta/url"
	"github.com/hashicorp/yamux"
	"github.com/rs/zerolog"
	"nhooyr.io/websocket"
)

type AcceptConfig struct {
	Type   string
	Agent  *url.URL
	Facade *url.URL
	Listen *url.URL
}

type AccessConfig struct {
	Type  string
	Agent *url.URL
	Url   *url.URL
}

type Accept struct {
	logger *zerolog.Logger
	assume func(network, address string) (router.Endpoint, error)
	config *AcceptConfig
	stop   func() error
}

type Access struct {
	logger *zerolog.Logger
	assume func(network, address string) (router.Endpoint, error)
	config *AccessConfig
	stop   func() error
}

func BuildAccept(name string, builder interface {
	spine.LoggerBuilder
	spine.EndpointBuilder
}) (spine.Service, error) {
	return &Accept{
		logger: builder.Logger(name),
		assume: builder.Endpoint,
		config: &AcceptConfig{
			Type: name,
		},
	}, nil
}

func BuildAccess(name string, builder interface {
	spine.LoggerBuilder
	spine.EndpointBuilder
}) (spine.Service, error) {
	return &Access{
		logger: builder.Logger(name),
		assume: builder.Endpoint,
		config: &AccessConfig{
			Type: name,
		},
	}, nil
}

func (agent *Accept) Close() (err error) {
	agent.logger.Info().Msg("Close")
	if agent.stop != nil {
		err = agent.stop()
		if err == nil {
			agent.stop = nil
		}
	}
	return
}

func (agent *Access) Close() (err error) {
	agent.logger.Info().Msg("Close")
	if agent.stop != nil {
		err = agent.stop()
		if err == nil {
			agent.stop = nil
		}
	}
	return
}

func (agent *Accept) MarshalJSON() ([]byte, error) {
	agent.logger.Info().Msg("MarshalJSON")
	return json.Marshal(agent.config)
}

func (agent *Access) MarshalJSON() ([]byte, error) {
	agent.logger.Info().Msg("MarshalJSON")
	return json.Marshal(agent.config)
}

func (agent *Accept) UnmarshalJSON(data []byte) (err error) {
	agent.logger.Info().Msg("UnmarshalJSON")
	var config AcceptConfig
	if err = json.Unmarshal(data, &config); err != nil {
		return
	}

	if agent.config.Type != config.Type {
		return spine.ErrServiceType
	}

	var needRestart bool

	if agent.config.Agent == nil || agent.config.Agent.String() != config.Agent.String() {
		needRestart = true
	}

	if agent.config.Listen == nil || agent.config.Listen.String() != config.Listen.String() {
		needRestart = true
	}

	// TODO: add dial/listen/shutdown timeout config

	agent.config = &config

	if needRestart {
		err = agent.start()
	}
	return
}

func (agent *Access) UnmarshalJSON(data []byte) (err error) {
	agent.logger.Info().Msg("UnmarshalJSON")
	var config AccessConfig
	if err = json.Unmarshal(data, &config); err != nil {
		return
	}

	if agent.config.Type != config.Type {
		return spine.ErrServiceType
	}

	var needRestart bool

	if agent.config.Agent == nil || agent.config.Agent.String() != config.Agent.String() {
		needRestart = true
	}

	if agent.config.Url == nil || agent.config.Url.String() != config.Url.String() {
		needRestart = true
	}

	// TODO: add dial/listen/shutdown timeout config

	agent.config = &config

	if needRestart {
		err = agent.start()
	}
	return
}

func (agent *Accept) start() (err error) {
	agent.logger.Info().Msg("start")

	if agent.stop != nil {
		err = agent.stop()
		if err != nil {
			return
		}
	}

	logger := agent.logger
	config := agent.config
	assume := agent.assume

	listen, err := net.Listen("tcp", config.Listen.Host)
	if err != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if !isWebsocket(r) {
			logger.Debug().Str("Method", r.Method).Str("URL", r.URL.String()).Msg("Not a websocket request")
			fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.String()))
			return
		}

		ws_conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			logger.Err(err).Msg("websocket.Accept failed")
			return
		}
		defer ws_conn.Close(websocket.StatusInternalError, "ws failed")

		conn := websocket.NetConn(ctx, ws_conn, websocket.MessageBinary)
		session, err := yamux.Server(conn, nil)
		if err != nil {
			logger.Err(err).Msg("yamux.Server failed")
			return
		}

		err = connect(ctx, session, assume, config.Agent, logger)
		if err != nil {
			logger.Warn().Err(err).Msg("connect failed")
			return
		}

		ws_conn.Close(websocket.StatusNormalClosure, "")
	})

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
		return
	}

	go server.Serve(listen)

	return
}

func (agent *Access) start() (err error) {
	agent.logger.Info().Msg("start")

	if agent.stop != nil {
		err = agent.stop()
		if err != nil {
			return
		}
	}

	logger := agent.logger
	config := agent.config
	assume := agent.assume

	ctx, cancel := context.WithCancel(context.Background())

	dial := func() (err error) {
		logger.Debug().Str("Url", config.Url.String()).Msg("connect to remote begin")
		defer logger.Debug().Str("Url", config.Url.String()).Msg("connect to remote end")
		ws_conn, response, err := websocket.Dial(ctx, config.Url.String(), nil)
		if err != nil {
			logger.Err(err).Msg("websocket.Dial failed")
			return
		}
		defer ws_conn.Close(websocket.StatusInternalError, "ws dial failed")
		_ = response // TODO: parse response

		conn := websocket.NetConn(ctx, ws_conn, websocket.MessageBinary)

		session, err := yamux.Client(conn, nil)
		if err != nil {
			logger.Err(err).Msg("yamux.Client failed")
			return
		}

		logger.Debug().Str("Url", config.Url.String()).Msg("connect")
		err = connect(ctx, session, assume, config.Agent, logger)
		if err != nil {
			logger.Warn().Err(err).Str("Url", config.Url.String()).Msg("connect failed")
			return
		}

		ws_conn.Close(websocket.StatusNormalClosure, "")
		return
	}

	agent.stop = func() (err error) {
		logger.Info().Msg("stop")
		cancel()
		return
	}

	go backoff.RetryNotify(
		dial,
		backoff.WithContext(NewNeverStopBackOff(), ctx),
		func(e error, d time.Duration) {
			logger.Warn().Err(e).Dur("duration", d).Msg("retry")
		},
	)
	return
}

func headerContains(header []string, value string) bool {
	for _, h := range header {
		for _, v := range strings.Split(h, ",") {
			if strings.EqualFold(strings.TrimSpace(v), value) {
				return true
			}
		}
	}

	return false
}

func isWebsocket(r *http.Request) bool {
	return r.Method == http.MethodGet &&
		headerContains(r.Header["Connection"], "upgrade") &&
		headerContains(r.Header["Upgrade"], "websocket")
}

func connect(ctx context.Context, session *yamux.Session, assume func(network, address string) (router.Endpoint, error), agentAddr *url.URL, logger *zerolog.Logger) (err error) {
	logger.Debug().Msg("connect begin")
	defer logger.Debug().Msg("connect end")

	sessionAddr := session.Addr()
	remote, err := assume(sessionAddr.Network(), sessionAddr.String())
	if err != nil {
		return
	}
	defer remote.Close()

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)

	server := &socket.Server{
		BaseContext: func(l net.Listener) context.Context { return ctx },
	}
	defer server.Close()

	toLocal := socket.NewForwardHandler(func(socket socket.Socket) (conn net.Conn, err error) {
		return remote.Dial(socket, agentAddr.Scheme, agentAddr.Host)
	})

	go func() {
		err := server.Serve(session, toLocal)
		if err != nil {
			logger.Warn().Err(err).Msg("toLocal done")
		} else {
			logger.Debug().Msg("toLocal done")
		}
		cancel()
	}()

	remoteURL, err := AccessAgent(&http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
				return session.Open()
			},
			ForceAttemptHTTP2:     false,
			MaxIdleConns:          1,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: time.Second * 10,
	}, agentAddr.Host).GetAgentURL()
	if err != nil {
		return
	}

	local, err := assume(remoteURL.Scheme, remoteURL.Host)
	if err != nil {
		return
	}
	// defer local.Close()

	toRemote := socket.NewForwardHandler(func(socket socket.Socket) (conn net.Conn, err error) {
		return session.Open()
	})

	go func() {
		err := server.Serve(local, toRemote)
		if err != nil {
			logger.Warn().Err(err).Msg("toRemote done")
		} else {
			logger.Debug().Msg("toRemote done")
		}
		cancel()
	}()

	logger.Debug().Str("Url", remoteURL.String()).Msg("connected")

	//TODO: add heartbeat when no traffic (yamux has internal loop, so follow yamux.accept and close all)

	<-ctx.Done()
	err = ctx.Err()
	return
}

func NewNeverStopBackOff() *backoff.ExponentialBackOff {
	b := &backoff.ExponentialBackOff{
		InitialInterval:     backoff.DefaultInitialInterval,
		RandomizationFactor: backoff.DefaultRandomizationFactor,
		Multiplier:          backoff.DefaultMultiplier,
		MaxInterval:         backoff.DefaultMaxInterval,
		MaxElapsedTime:      0,
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	}
	b.Reset()
	return b
}
