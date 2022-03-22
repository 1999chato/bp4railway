package hub

import (
	"bypaths/intranet"
	"bypaths/notary"
	"bypaths/socket"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/hashicorp/yamux"
	"github.com/rs/zerolog"
	"nhooyr.io/websocket"
)

type Access struct {
	Domain string

	Verify string // default: "ed25519"
	Secret string // ed25519 public key
	Sign   string

	Dial   string
	Listen string
}

func (access *Access) String() string {
	return access.Domain + ":" + access.Listen + ":" + access.Dial
}

type Hub struct {
	Notarize *notary.Notarize
	intranet *intranet.Intranet
	Logger   *zerolog.Logger
}

func NewHub() *Hub {
	return &Hub{
		intranet: &intranet.Intranet{},
	}
}

func (s *Hub) Connect(access *Access) (conn net.Conn, err error) {
	return
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
	stat, err := s.Notarize.GetStatementFromToken(token)
	if err != nil {
		s.response(w, http.StatusUnauthorized, nil, "")
		logger.Err(err).Str("token", token).Msg(http.StatusText(http.StatusUnauthorized))
		return
	}

	logger = logger.With().Stringer("stat", stat).Logger()

	logger.Debug().Msg("handle websocket")
	if !IsWebsocketRequest(r) {
		if access.Listen == "" && access.Dial == "" {
			s.response(w, http.StatusOK, nil, "")
			return
		}
		s.response(w, http.StatusOK, nil, "")
		logger.Err(errors.New("not a websocket request")).Interface("header", r.Header).Msg(http.StatusText(http.StatusBadRequest))
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

		server := &socket.Server{
			BaseContext: func(l net.Listener) context.Context { return ctx },
		}
		defer server.Close()

		go func() {
			err := server.Serve(session, toLocal)
			if err != nil {
				logger.Warn().Err(err).Msg("toLocal done")
			} else {
				logger.Debug().Msg("toLocal done")
			}
			cancel()
		}()

		targetURL := identity.URL
		target := s.intranet.Assume(targetURL.Scheme, targetURL.Host)

		toRemote := socket.NewForwardHandler(func(socket *socket.Context) (conn net.Conn, err error) {
			return session.Open()
		})

		go func() {
			err := server.Serve(target, toRemote)
			if err != nil {
				logger.Warn().Err(err).Msg("toLocal done")
			} else {
				logger.Debug().Msg("toLocal done")
			}
			cancel()
		}()

		//TODO: add heartbeat when no traffic (yamux has internal loop, so follow yamux.accept and close all)

		<-ctx.Done()
		err = ctx.Err()

	}

	ws_conn.Close(websocket.StatusNormalClosure, "")
}
