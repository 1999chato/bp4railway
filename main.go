package main

import (
	"bypaths/hub"
	"bypaths/intranet"
	"bypaths/notary"
	"bypaths/proxy"
	"fmt"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
)

func logRequest(h http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		log.Printf("req Method:%v Scheme:%+v Host:%v IsAbs:%v", r.Method, r.URL.Scheme, r.URL.Host, r.URL.IsAbs())
		h.ServeHTTP(rw, r)
	})
}

func hello(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "hello %v, URL:%v\n", req.RemoteAddr, req.URL.String())
}

func headers(w http.ResponseWriter, req *http.Request) {
	for name, headers := range req.Header {
		for _, h := range headers {
			fmt.Fprintf(w, "%v: %v\n", name, h)
		}
	}
}

func handleHub() http.Handler {
	inet := &intranet.Intranet{}
	proxy := &proxy.HttpProxy{}
	go http.Serve(inet, logRequest(proxy))

	return &hub.Hub{
		Notarize: &notary.Notarize{
			State: &notary.LocalState{},
		},
		Intranet: inet,
		Logger:   &log.Logger,
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Listening on port %v\n", port)

	http.HandleFunc("/", hello)
	http.HandleFunc("/headers", headers)

	http.Handle("/xxoo", handleHub())

	http.ListenAndServe(":"+port, nil)
}
