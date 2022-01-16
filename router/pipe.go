package router

import "net"

func Pipe(src, dst net.Addr) (net.Conn, net.Conn) {
	return net.Pipe()
}
