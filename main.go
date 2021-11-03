package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
)

func main() {
	fmt.Println("hi railway")
	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = "8090"
	}

	fmt.Println("port is " + port)

	proxyHandler := &HttpProxy{}

	logMiddleware := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			log.Printf("req Method:%v Scheme:%+v Host:%v IsAbs:%v headers:%v", r.Method, r.URL.Scheme, r.URL.Host, r.URL.IsAbs(), r.Header)
			h.ServeHTTP(rw, r)
		})
	}

	noProxyMiddleware := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			if r.URL.IsAbs() || r.Method == http.MethodConnect {
				h.ServeHTTP(rw, r)
			} else {
				rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
				rw.Header().Set("X-Content-Type-Options", "nosniff")
				rw.WriteHeader(200)
				fmt.Fprintln(rw, "no")
			}
		})
	}

	http.ListenAndServe(":"+port, logMiddleware(noProxyMiddleware(proxyHandler)))
}

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
