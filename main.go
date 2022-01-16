package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dacapoday/server-meta/router"
	"github.com/dacapoday/server-meta/spine"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Builder = interface {
	spine.LoggerBuilder
	spine.EndpointBuilder
	spine.ServiceBuilder
}

type BuildService = func(string, Builder) (spine.Service, error)

type Base struct {
	logger        *zerolog.Logger
	router        *router.Router
	buildServices map[string]BuildService
}

func (base *Base) Logger(name string) *zerolog.Logger {
	logger := base.logger.With().Str("logger", name).Logger()
	return &logger
}

func (base *Base) Endpoint(network, address string) (router.Endpoint, error) {
	return base.router.Listen(context.Background(), network, address)
}

func (base *Base) Service(name string) (spine.Service, error) {
	buildService, ok := base.buildServices[name]
	if !ok {
		return nil, fmt.Errorf("%s servie not found", name)
	}
	return buildService(name, base)
}

var base *Base

func init() {
	base = &Base{
		logger: &log.Logger,
		router: &router.Router{},
		buildServices: map[string]BuildService{
			"Group": func(name string, builder Builder) (spine.Service, error) {
				return BuildGroup(name, builder)
			},
			"Agent": func(name string, builder Builder) (spine.Service, error) {
				return BuildAgent(name, builder)
			},
			"Accept": func(name string, builder Builder) (spine.Service, error) {
				return BuildAccept(name, builder)
			},
			"Access": func(name string, builder Builder) (spine.Service, error) {
				return BuildAccess(name, builder)
			},
			"Forward": func(name string, builder Builder) (spine.Service, error) {
				return BuildTcpForward(name, builder)
			},
			"Proxy": func(name string, builder Builder) (spine.Service, error) {
				return BuildHttpProxy(name, builder)
			},
		},
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		return
	}

	logger := base.Logger("main")
	logger.Info().Msg("start")
	defer logger.Info().Msg("end")

	data := []byte(`
	{
		"Type":"Group",
		"Services":{
			"agent":{
				"Type":"Agent",
				"Agent":"agent://server.agent"
			},
			"accept":{
				"Type":"Accept",
				"Agent":"agent://server.agent",
				"Listen":"tcp://0.0.0.0:` + port + `"
			}
		}
	}
	`)

	service, err := base.Service("Group")
	if err != nil {
		logger.Error().Err(err).Msg("service init")
		return
	}
	logger.Info().Msg("service init")

	err = service.UnmarshalJSON(data)
	if err != nil {
		logger.Error().Err(err).Msg("service start")
		return
	}
	logger.Info().Msg("service start")

	{
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)
		<-osSignals
	}

	err = service.Close()
	if err != nil {
		logger.Err(err).Msg("service close failed")
		return
	}
	logger.Info().Msg("service close")
}
