package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	m "github.com/dacapoday/marshallable"
	httpserver "github.com/dacapoday/server-meta/http"
	"github.com/dacapoday/server-meta/intranet"
	"github.com/dacapoday/server-meta/proxy"
	"github.com/dacapoday/server-meta/request"
	"github.com/dacapoday/server-meta/spine"
	"github.com/rs/zerolog"

	"github.com/dghubble/sling"
)

type pattern struct {
	Pattern *m.Regexp `json:",omitempty"`
	Replace string    `json:",omitempty"`
}

func isSameSlicePattern(a, b []pattern) bool {
	if (a == nil) != (b == nil) {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i].Pattern.String() != b[i].Pattern.String() {
			return false
		}
		if a[i].Replace != b[i].Replace {
			return false
		}
	}

	return true
}

type AgentConfig struct {
	Type     string
	Agent    *m.URL
	Patterns []pattern
	Timeout  time.Duration
}

type Agent struct {
	logger *zerolog.Logger
	assume func(network, address string) (intranet.Endpoint, error)
	config *AgentConfig
	stop   func() error
}

func BuildAgent(name string, builder interface {
	spine.LoggerBuilder
	spine.EndpointBuilder
}) (spine.Service, error) {
	return &Agent{
		logger: builder.Logger(name),
		assume: builder.Endpoint,
		config: &AgentConfig{
			Type: name,
		},
	}, nil
}

func (agent *Agent) Close() (err error) {
	agent.logger.Info().Msg("Close")
	if agent.stop != nil {
		err = agent.stop()
		if err == nil {
			agent.stop = nil
		}
	}
	return
}

func (agent *Agent) MarshalJSON() ([]byte, error) {
	agent.logger.Info().Msg("MarshalJSON")
	return json.Marshal(agent.config)
}

func (agent *Agent) UnmarshalJSON(data []byte) (err error) {
	agent.logger.Info().Msg("UnmarshalJSON")
	var config AgentConfig
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

	if !isSameSlicePattern(agent.config.Patterns, config.Patterns) {
		needRestart = true
	}

	if len(config.Patterns) == 0 {
		p := m.MustRegexp(`\w+://(?:\S+\.)?(\w+\.(agent))`)
		config.Patterns = append(config.Patterns, pattern{
			Pattern: &p,
			Replace: "$2://$1",
		})
		needRestart = true
	}

	// TODO: add dial/listen/shutdown timeout config

	agent.config = &config

	if needRestart {
		err = agent.start()
	}
	return
}

func (agent *Agent) start() (err error) {
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

	dialer := &net.Dialer{}

	dial := func(ctx context.Context, network string, addr string) (net.Conn, error) {
		target := network + "://" + addr

		for _, pattern := range config.Patterns {
			if pattern.Pattern.MatchString(target) {
				if pattern.Replace == "" {
					logger.Debug().Str("target", target).Msg("dial match pattern")
					return endpoint.Dial(ctx, network, addr)
				}

				if replace, err := url.Parse(pattern.Pattern.ReplaceAllString(target, pattern.Replace)); err == nil {
					logger.Debug().Str("target", target).Str("replace", replace.String()).Msg("dial match pattern and replace")
					return endpoint.Dial(ctx, replace.Scheme, replace.Host)
				} else {
					logger.Warn().Err(err).Str("target", target).Msg("dial match pattern but replace failed")
					continue
				}
			}
		}

		logger.Debug().Str("target", target).Msg("dial not match pattern")
		return dialer.DialContext(ctx, network, addr)
	}

	handler := newAgentHandler(config.Agent.URL, dial, logger)

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

	go server.Serve(endpoint, handler)
	return
}

func newAgentHandler(addr *url.URL, dial func(ctx context.Context, network string, addr string) (conn net.Conn, err error), logger *zerolog.Logger) http.HandlerFunc {
	proxy := &proxy.HttpProxy{
		Transport: &http.Transport{
			DialContext: dial,
		},
	}

	router := http.NewServeMux()
	router.HandleFunc("/api/address", request.GET(func(w http.ResponseWriter, r *http.Request) {
		traceID := request.GetTraceID(w, r)
		logger.Info().Str("traceID", traceID).Str("Addr", addr.String()).Msg("handleGetAddr")
		request.ResponseJSON(w, 200, Data{Data: struct{ Agent string }{
			Agent: addr.String(),
		}})
	}))
	router.HandleFunc("/api/hardware", request.GET(func(w http.ResponseWriter, r *http.Request) {
		traceID := request.GetTraceID(w, r)
		logger.Info().Str("traceID", traceID).Msg("handleGetHardware")
		request.ResponseJSON(w, 200, Data{Data: struct{ Hardware string }{
			Hardware: "None",
		}})
	}))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := request.GetTraceID(w, r)
		logger.Info().Str("traceID", traceID).Str("Method", r.Method).Str("URL", r.URL.String()).Msg("ServeHTTP")
		if proxy.IsProxyRequest(r) {
			logger.Info().Str("traceID", traceID).Msg("proxy.ServeHTTP")
			proxy.ServeHTTP(w, r)
			return
		}
		router.ServeHTTP(w, r)
	})
}

type Data = struct{ Data interface{} }

type AgentClient func() *sling.Sling

func AccessAgent(doer sling.Doer, agentHost string) AgentClient {
	base := sling.New().
		Base("http://" + agentHost).
		Doer(doer)
	return AgentClient(func() *sling.Sling {
		return base.New()
	})
}

func (agent AgentClient) GetAgentURL() (addr *url.URL, err error) {
	var data struct{ Data struct{ Agent string } }
	resp, err := agent().
		Get("api/address").
		ReceiveSuccess(&data)

	if err != nil {
		err = fmt.Errorf("agent.GetAgentURL: %w", err)
		return
	}
	if code := resp.StatusCode; code > 299 {
		err = fmt.Errorf("agent.GetAgentURL code: %d", code)
		return
	}

	addr, err = url.Parse(data.Data.Agent)
	if err != nil {
		err = fmt.Errorf("agent.GetAgentURL: %w", err)
		return
	}

	return
}
