package main

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/dacapoday/server-meta/spine"
	"github.com/rs/zerolog"
)

type Group struct {
	logger   *zerolog.Logger
	build    func(name string) (spine.Service, error)
	Type     string
	Services map[string]spine.Service
}

func BuildGroup(name string, builder interface {
	spine.LoggerBuilder
	spine.ServiceBuilder
}) (spine.Service, error) {
	return &Group{
		logger: builder.Logger(name),
		build:  builder.Service,
		Type:   name,
	}, nil
}

func (group *Group) Close() (err error) {
	wg := sync.WaitGroup{}
	for _, service := range group.Services {
		wg.Add(1)
		go func(service spine.Service) {
			if err := service.Close(); err != nil {
				group.logger.Err(err).Msg("service.Close")
			}
			wg.Done()
		}(service)
	}
	wg.Wait()
	return
}

func (group *Group) MarshalJSON() ([]byte, error) {
	group.logger.Info().Msg("MarshalJSON")
	return json.Marshal(struct {
		Type     string                   `json:",omitempty"`
		Services map[string]spine.Service `json:",omitempty"`
	}{
		Type:     group.Type,
		Services: group.Services,
	})
}

func (group *Group) UnmarshalJSON(data []byte) error {
	group.logger.Info().Msg("UnmarshalJSON")

	var payload struct {
		Type     string
		Services map[string]json.RawMessage
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	if group.Type != payload.Type {
		return spine.ErrServiceType
	}

	for key, service := range group.Services {
		if _, ok := payload.Services[key]; !ok {
			if err := service.Close(); err != nil {
				return err
			}
			delete(group.Services, key)
		}
	}
	if group.Services == nil {
		group.Services = make(map[string]spine.Service)
	}
	for key, data := range payload.Services {
		if service, ok := group.Services[key]; ok {
			var err error
			if err = service.UnmarshalJSON(data); err == nil {
				continue
			}
			if !errors.Is(err, spine.ErrServiceType) {
				return err
			}
			if err = service.Close(); err != nil {
				return err
			}
		}

		var payload struct{ Type string }
		if err := json.Unmarshal(data, &payload); err != nil {
			return err
		}

		service, err := group.build(payload.Type)
		if err != nil {
			return err
		}
		group.Services[key] = service

		if err := service.UnmarshalJSON(data); err != nil {
			return err
		}
	}
	return nil
}
