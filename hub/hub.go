package hub

import (
	"bypaths/intranet"
	"bypaths/notary"
	"bypaths/socket"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/hashicorp/yamux"
	"github.com/rs/zerolog"
	"nhooyr.io/websocket"
)

type Policy struct {
	Dial   string
	Listen string
}

type Hub struct {
	Notarize *notary.Notarize
	Intranet *intranet.Intranet
	Logger   *zerolog.Logger
}

func NewHub() *Hub {
	return &Hub{
		Notarize: &notary.Notarize{
			State: &notary.LocalState{},
		},
		Intranet: &intranet.Intranet{},
	}
}

func (s *Hub) response(w http.ResponseWriter, status int, body []byte, contentType string) {
	w.WriteHeader(status)
	if body != nil {
		if contentType == "" {
			contentType = "text/plain; charset=utf-8"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fmt.Fprintln(w, body)
	}
}

func (s *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	traceID := GetTraceIdFromRequest(r)
	logger := s.Logger.With().Str("logger", "hub").Str("traceID", traceID).Logger()

	logger.Debug().Msg("handle GetTokenFromRequest")
	token := GetTokenFromRequest(r)
	if token == "" {
		s.response(w, http.StatusUnauthorized, nil, "")
		logger.Err(errors.New("no token in request")).Msg(http.StatusText(http.StatusUnauthorized))
		return
	}

	logger.Debug().Msg("handle DecodeToken")
	Payload, Domain, _, _, _, _, err := s.Notarize.DecodeToken(token)
	if err != nil {
		s.response(w, http.StatusUnauthorized, nil, "")
		logger.Err(err).Str("token", token).Msg(http.StatusText(http.StatusUnauthorized))
		return
	}

	logger = logger.With().Bytes("Payload", Payload).Str("Domain", Domain).Logger()

	logger.Debug().Msg("handle DecodePayload")
	var policy Policy
	err = json.Unmarshal(Payload, &policy)
	if err != nil {
		s.response(w, http.StatusBadRequest, nil, "")
		logger.Err(err).Msg("invalid payload")
		return
	}

	logger.Debug().Msg("handle websocket")
	if !IsWebsocketRequest(r) {
		if policy.Listen != "" || policy.Dial != "" {
			s.response(w, http.StatusBadRequest, nil, "")
			logger.Error().Msg("not a websocket request")
			return
		}
		s.response(w, http.StatusOK, nil, "")
		return
	}

	ws_conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		logger.Err(err).Msg("Websocket Accept Failed")
		return
	}
	defer ws_conn.Close(websocket.StatusInternalError, "")

	logger.Debug().Msg("handle yamux")
	conn := websocket.NetConn(ctx, ws_conn, websocket.MessageBinary)
	session, err := yamux.Server(conn, nil)
	if err != nil {
		logger.Err(err).Msg("yamux failed")
		return
	}

	{
		logger.Info().Msg("connect begin")
		defer logger.Info().Msg("connect end")

		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		defer cancel()

		server := &socket.Server{
			BaseContext: func(l net.Listener) context.Context { return ctx },
		}
		defer server.Close()

		if policy.Dial != "" {
			policyDial := socket.NewForwardHandler(func(socket *socket.Context) (conn net.Conn, err error) {
				return s.Intranet.Dial(socket, Domain, policy.Dial)
			})

			go func() {
				err := server.Serve(session, policyDial)
				if err != nil {
					logger.Warn().Err(err).Msg("policy Dial done")
				} else {
					logger.Debug().Msg("policy Dial done")
				}
				cancel()
			}()
		}

		if policy.Listen != "" {
			policyListen := socket.NewForwardHandler(func(socket *socket.Context) (conn net.Conn, err error) {
				return session.Open()
			})
			target := s.Intranet.Assume(Domain, policy.Listen)
			go func() {
				err := server.Serve(target, policyListen)
				if err != nil {
					logger.Warn().Err(err).Msg("toLocal done")
				} else {
					logger.Debug().Msg("toLocal done")
				}
				cancel()
			}()
		}

		logger.Info().Msg("connected")

		//TODO: add heartbeat when no traffic (yamux has internal loop, so follow yamux.accept and close all)

		if policy.Listen != "" || policy.Dial != "" {
			<-ctx.Done()
			err = ctx.Err()
		}
	}

	ws_conn.Close(websocket.StatusNormalClosure, "")
}
