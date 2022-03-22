package intranet

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIntranetBasic(t *testing.T) {
	intranet := &Intranet{}

	a_conn := make(chan net.Conn)
	a_err := make(chan error)
	go func() {
		conn, err := intranet.Accept()
		a_err <- err
		a_conn <- conn
	}()

	time.Sleep(time.Millisecond * 100)

	timeout, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, err := intranet.Dial(timeout, "h", "i")
	assert.NoError(t, err)
	assert.NotNil(t, conn)

	assert.NoError(t, <-a_err)
	assert.NotNil(t, <-a_conn)

	ls := intranet.Lookup("")
	for _, v := range ls {
		t.Logf("n:%v,a:%v\n", v.Network(), v.String())
	}

	{
		timeout, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		conn, err := intranet.Dial(timeout, "h", "i")
		t.Log(err)
		assert.Error(t, err)
		assert.Nil(t, conn)
	}

	err = intranet.Close()
	assert.NoError(t, err)
	{
		timeout, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		conn, err := intranet.Dial(timeout, "h", "i")
		t.Log(err)
		assert.Error(t, err)
		assert.Nil(t, conn)
	}
}

func TestIntranetAssumeBasic(t *testing.T) {
	intranet := &Intranet{
		Singularity: Addr{"x", "x"},
	}
	e := intranet.Assume("h", "i")
	assert.Equal(t, e.Addr().Network(), "h")
	assert.Equal(t, e.Addr().String(), "i")
	assert.Equal(t, intranet.Addr().Network(), "x")
	assert.Equal(t, intranet.Addr().String(), "x")
	a_conn := make(chan net.Conn)
	a_err := make(chan error)
	go func() {
		conn, err := e.Accept()
		a_err <- err
		a_conn <- conn
	}()
	time.Sleep(time.Millisecond * 100)
	{
		timeout, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		conn, err := intranet.Dial(timeout, "h", "i")
		t.Log(err)
		assert.NoError(t, err)
		assert.NotNil(t, conn)
		assert.NoError(t, <-a_err)
		assert.NotNil(t, <-a_conn)
	}
	go func() {
		conn, err := e.Accept()
		a_err <- err
		a_conn <- conn
	}()
	time.Sleep(time.Millisecond * 100)
	e.Close()
	assert.Error(t, <-a_err)
	assert.Nil(t, <-a_conn)

	{
		conn, err := e.Dial(context.Background(), "h", "i")
		assert.Error(t, err)
		assert.Nil(t, conn)
	}

	{
		conn, err := e.Accept()
		assert.Error(t, err)
		assert.Nil(t, conn)
	}

	go func() {
		conn, err := intranet.Accept()
		a_err <- err
		a_conn <- conn
	}()
	time.Sleep(time.Millisecond * 100)
	intranet.Close()
	assert.Error(t, <-a_err)
	assert.Nil(t, <-a_conn)
}
