package intranet

import "net"

type conn struct {
	net.Conn
	local  net.Addr
	remote net.Addr
}

func (conn *conn) LocalAddr() net.Addr {
	return conn.local
}

func (conn *conn) RemoteAddr() net.Addr {
	return conn.remote
}

func Pipe(src, dst net.Addr) (net.Conn, net.Conn) {
	s, d := net.Pipe()
	return &conn{s, src, dst}, &conn{d, dst, src}
}
