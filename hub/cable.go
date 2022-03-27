package hub

import (
	"context"
	"net"
	"net/http"

	"github.com/hashicorp/yamux"
	"nhooyr.io/websocket"
)

type Cable struct {
	*yamux.Session
	wsConn *websocket.Conn
}

func (c *Cable) Close() (err error) {
	err = c.Session.Close()
	c.wsConn.Close(websocket.StatusNormalClosure, "")
	return
}

func (c *Cable) Addr() net.Addr { return c.Session.Addr() }

func ConnectHub(ctx context.Context, url string, token string) (cable *Cable, err error) {
	ws_conn, response, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": {"Bearer " + token},
		},
	})
	if err != nil {
		return
	}
	_ = response // TODO: parse response

	conn := websocket.NetConn(ctx, ws_conn, websocket.MessageBinary)
	session, err := yamux.Client(conn, nil)
	if err != nil {
		ws_conn.Close(websocket.StatusInternalError, "yamux failed")
		return
	}

	cable = &Cable{Session: session, wsConn: ws_conn}
	return
}
