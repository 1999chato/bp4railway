package spine

import (
	"github.com/dacapoday/server-meta/router"
	"github.com/rs/zerolog"
)

type LoggerBuilder interface {
	Logger(name string) *zerolog.Logger
}

type ServiceBuilder interface {
	Service(name string) (Service, error)
}

type EndpointBuilder interface {
	Endpoint(network, address string) (router.Endpoint, error)
}
