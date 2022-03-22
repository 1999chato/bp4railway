package hub

import (
	"net/http"
	"strings"

	uuid "github.com/satori/go.uuid"
)

func headerContains(header []string, value string) bool {
	for _, h := range header {
		for _, v := range strings.Split(h, ",") {
			if strings.EqualFold(strings.TrimSpace(v), value) {
				return true
			}
		}
	}

	return false
}

func IsWebsocketRequest(r *http.Request) bool {
	return r.Method == http.MethodGet &&
		headerContains(r.Header["Connection"], "upgrade") &&
		headerContains(r.Header["Upgrade"], "websocket")
}

func GetTraceIdFromRequest(r *http.Request) string {
	traceIDToken := "X-Trace-Id"
	traceID := r.Header.Get(traceIDToken)
	if traceID == "" {
		traceID = uuid.NewV4().String()
		r.Header.Set(traceIDToken, traceID)
	}
	return traceID
}

func GetTokenFromRequest(r *http.Request) string {
	bearer := r.Header.Get("Authorization")
	if len(bearer) > 7 && strings.ToUpper(bearer[0:6]) == "BEARER" {
		return bearer[7:]
	}
	return ""
}
