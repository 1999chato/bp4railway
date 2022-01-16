package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
)

type HttpProxy httputil.ReverseProxy

func (p *HttpProxy) IsProxyRequest(req *http.Request) bool {
	return req.URL.IsAbs() || req.Method == http.MethodConnect
}

func (p *HttpProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodConnect {
		if p.Director == nil {
			p.Director = func(req *http.Request) {}
		}
		(*httputil.ReverseProxy)(p).ServeHTTP(rw, req)
		return
	} else if p.Director != nil {
		p.Director(req)
	}

	hj, ok := rw.(http.Hijacker)
	if !ok {
		p.getErrorHandler()(rw, req, fmt.Errorf("can't %s using non-Hijacker ResponseWriter type %T", req.Method, rw))
		return
	}

	// TODO: check port

	backConn, err := p.dial(req.Context(), "tcp", req.URL.Host)
	if err != nil {
		p.getErrorHandler()(rw, req, fmt.Errorf("dial failed on %s: %v", req.Method, err))
		return
	}

	backConnCloseCh := make(chan bool)
	go func() {
		select {
		case <-req.Context().Done():
		case <-backConnCloseCh:
		}
		backConn.Close()
	}()

	defer close(backConnCloseCh)

	conn, _, err := hj.Hijack()
	if err != nil {
		p.getErrorHandler()(rw, req, fmt.Errorf("hijack failed on %s: %v", req.Method, err))
		return
	}
	defer conn.Close()

	_, err = conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		p.getErrorHandler()(rw, req, fmt.Errorf("write failed on %s: %v", req.Method, err))
		return
	}

	errc := make(chan error, 1)
	cc := connectCopier{user: conn, backend: backConn}
	go cc.copyToBackend(errc)
	go cc.copyFromBackend(errc)
	<-errc
}

//see go1.17.1:src/net/http/httputil/reverseproxy.go:616 switchProtocolCopier
type connectCopier struct {
	user, backend io.ReadWriter
}

func (c connectCopier) copyFromBackend(errc chan<- error) {
	_, err := io.Copy(c.user, c.backend)
	errc <- err
}

func (c connectCopier) copyToBackend(errc chan<- error) {
	_, err := io.Copy(c.backend, c.user)
	errc <- err
}

func (p *HttpProxy) logf(format string, args ...interface{}) {
	if p.ErrorLog != nil {
		p.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

func (p *HttpProxy) defaultErrorHandler(rw http.ResponseWriter, req *http.Request, err error) {
	p.logf("http: proxy error: %v", err)
	rw.WriteHeader(http.StatusBadGateway)
}

func (p *HttpProxy) getErrorHandler() func(http.ResponseWriter, *http.Request, error) {
	if p.ErrorHandler != nil {
		return p.ErrorHandler
	}
	return p.defaultErrorHandler
}

var zeroDialer net.Dialer

func (p *HttpProxy) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	if t, ok := p.Transport.(*http.Transport); ok {
		if t.DialContext != nil {
			return t.DialContext(ctx, network, addr)
		}
	}
	return zeroDialer.DialContext(ctx, network, addr)
}
