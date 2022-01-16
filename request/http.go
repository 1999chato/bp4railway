package request

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"strings"

	uuid "github.com/satori/go.uuid"
)

func GET(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handler(w, r)
	}
}

func GetTraceID(w http.ResponseWriter, r *http.Request) string {
	TRACEID := "X-Trace-Id"
	traceID := w.Header().Get(TRACEID)
	if traceID == "" {
		traceID = r.Header.Get(TRACEID)
		if traceID == "" {
			traceID = uuid.NewV4().String()
			w.Header().Set(TRACEID, traceID)
		}
	}
	return traceID
}

func HttpConnect(ctx context.Context, conn net.Conn, host string, header http.Header) (err error) {
	{
		var w io.Writer = conn

		var bw *bufio.Writer
		if _, ok := w.(io.ByteWriter); !ok {
			bw = bufio.NewWriter(w)
			w = bw
		}

		_, err = fmt.Fprintf(w, "CONNECT %s HTTP/1.1\r\n", host)
		if err != nil {
			return
		}

		if header == nil || header.Get("Host") == "" {
			_, err = fmt.Fprintf(w, "Host: %s\r\n", host)
			if err != nil {
				return
			}
		}

		if header != nil {
			err = header.Write(w)
			if err != nil {
				return
			}
		}

		_, err = io.WriteString(w, "\r\n")
		if err != nil {
			return
		}

		if bw != nil {
			err = bw.Flush()
			if err != nil {
				return
			}
		}
	}
	{
		br := bufio.NewReader(conn)
		tp := textproto.NewReader(br)

		var line string
		line, err = tp.ReadLine()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return
		}

		var statusCode string
		if i := strings.IndexByte(line, ' '); i == -1 {
			err = fmt.Errorf("malformed HTTP response %q", line)
			return
		} else {
			statusCode = strings.TrimLeft(line[i+1:], " ")
			if i = strings.IndexByte(statusCode, ' '); i != -1 {
				statusCode = statusCode[:i]
			}
		}
		if len(statusCode) != 3 {
			err = fmt.Errorf("malformed HTTP status code %q", statusCode)
			return
		}

		_, err = tp.ReadMIMEHeader()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return
		}

		if statusCode != "200" {
			err = fmt.Errorf("HTTP CONNECT failed with status code %q", statusCode)
			return
		}
	}
	return
}

func ResponseJSON(w http.ResponseWriter, code int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	b, err := json.Marshal(body)
	if err != nil {
		w.Write([]byte(`{"Error":"json.Marshal(body) failed"}`))
		return
	}
	w.Write(b)
}
