package main

import (
	"fmt"
	"net/http"
	"os"
)

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

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Listening on port %v\n", port)

	http.HandleFunc("/", hello)
	http.HandleFunc("/headers", headers)

	http.ListenAndServe(":"+port, nil)
}
