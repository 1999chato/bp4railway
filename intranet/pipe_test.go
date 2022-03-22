package intranet

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPipeBasic(t *testing.T) {
	s, d := Pipe(Addr{"from", "src"}, Addr{"to", "dst"})
	assert.NotNil(t, s)
	assert.NotNil(t, d)
	assert.Equal(t, s.LocalAddr().Network(), "from")
	assert.Equal(t, s.RemoteAddr().Network(), "to")
	assert.Equal(t, d.LocalAddr().Network(), "to")
	assert.Equal(t, d.RemoteAddr().Network(), "from")
	assert.Equal(t, s.LocalAddr().String(), "src")
	assert.Equal(t, s.RemoteAddr().String(), "dst")
	assert.Equal(t, d.LocalAddr().String(), "dst")
	assert.Equal(t, d.RemoteAddr().String(), "src")
}

func TestRouteBasic(t *testing.T) {
	intranet := &Intranet{
		Singularity: Addr{"x", "x"},
	}
	efrom := intranet.Assume("from", "src")
	eto := intranet.Assume("to", "dst")
	a_conn := make(chan net.Conn)
	a_err := make(chan error)
	go func() {
		conn, err := eto.Accept()
		a_err <- err
		a_conn <- conn
	}()
	time.Sleep(time.Millisecond * 100)
	{
		timeout, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		conn, err := efrom.Dial(timeout, "to", "dst")
		assert.NoError(t, err)
		assert.NotNil(t, conn)
		assert.NoError(t, <-a_err)
		d := <-a_conn
		assert.NotNil(t, d)
		s := conn

		assert.Equal(t, s.LocalAddr().Network(), "from")
		assert.Equal(t, s.RemoteAddr().Network(), "to")
		assert.Equal(t, d.LocalAddr().Network(), "to")
		assert.Equal(t, d.RemoteAddr().Network(), "from")
		assert.Equal(t, s.LocalAddr().String(), "src")
		assert.Equal(t, s.RemoteAddr().String(), "dst")
		assert.Equal(t, d.LocalAddr().String(), "dst")
		assert.Equal(t, d.RemoteAddr().String(), "src")
	}
}

func TestSingularityBasic(t *testing.T) {
	intranet := &Intranet{
		Singularity: Addr{"x", "x"},
	}
	efrom := intranet.Assume("from", "src")
	eto := intranet.Assume("to", "dst")
	a_conn := make(chan net.Conn)
	a_err := make(chan error)
	go func() {
		conn, err := intranet.Accept()
		a_err <- err
		a_conn <- conn
	}()
	time.Sleep(time.Millisecond * 100)
	{
		timeout, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		conn, err := efrom.Dial(timeout, "to", "dst")
		assert.NoError(t, err)
		assert.NotNil(t, conn)
		assert.NoError(t, <-a_err)
		d := <-a_conn
		assert.NotNil(t, d)
		s := conn

		assert.Equal(t, s.LocalAddr().Network(), "from")
		assert.Equal(t, s.RemoteAddr().Network(), "to")
		assert.Equal(t, d.LocalAddr().Network(), "to")
		assert.Equal(t, d.RemoteAddr().Network(), "from")
		assert.Equal(t, s.LocalAddr().String(), "src")
		assert.Equal(t, s.RemoteAddr().String(), "dst")
		assert.Equal(t, d.LocalAddr().String(), "dst")
		assert.Equal(t, d.RemoteAddr().String(), "src")
	}
	go func() {
		conn, err := eto.Accept()
		a_err <- err
		a_conn <- conn
	}()
	time.Sleep(time.Millisecond * 100)
	{
		timeout, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		conn, err := intranet.Dial(timeout, "to", "dst")
		assert.NoError(t, err)
		assert.NotNil(t, conn)
		assert.NoError(t, <-a_err)
		d := <-a_conn
		assert.NotNil(t, d)
		s := conn

		assert.Equal(t, s.LocalAddr().Network(), "x")
		assert.Equal(t, s.RemoteAddr().Network(), "to")
		assert.Equal(t, d.LocalAddr().Network(), "to")
		assert.Equal(t, d.RemoteAddr().Network(), "x")
		assert.Equal(t, s.LocalAddr().String(), "x")
		assert.Equal(t, s.RemoteAddr().String(), "dst")
		assert.Equal(t, d.LocalAddr().String(), "dst")
		assert.Equal(t, d.RemoteAddr().String(), "x")
	}
}
