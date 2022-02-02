package http

import (
	"context"
	"net"
	"net/http"
)

type Server http.Server

func (server *Server) Serve(listener net.Listener, handler http.Handler) error {
	server.Handler = handler
	return (*http.Server)(server).Serve(listener)
}

func (server *Server) Close() error {
	return (*http.Server)(server).Close()
}

func (server *Server) Shutdown(ctx context.Context) error {
	return (*http.Server)(server).Shutdown(ctx)
}
