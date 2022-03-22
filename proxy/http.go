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

	target, err := p.dial(req.Context(), "tcp", req.URL.Host)
	if err != nil {
		p.getErrorHandler()(rw, req, fmt.Errorf("dial failed on %s: %v", req.Method, err))
		return
	}
	defer target.Close()

	source, _, err := hj.Hijack()
	if err != nil {
		p.getErrorHandler()(rw, req, fmt.Errorf("hijack failed on %s: %v", req.Method, err))
		return
	}
	defer source.Close()

	_, err = source.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		p.getErrorHandler()(rw, req, fmt.Errorf("write failed on %s: %v", req.Method, err))
		return
	}

	done := make(chan struct{})
	go copy(source, target, done)
	go copy(target, source, done)

	select {
	case <-req.Context().Done():
	case done <- struct{}{}:
	}
	close(done)
}

func copy(to io.Writer, from io.Reader, done <-chan struct{}) {
	io.Copy(to, from)
	<-done
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
