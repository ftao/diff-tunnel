package main

import (
	"errors"
	"net"
	"time"
)

// TunnelConn
type TunnelConn struct {
	id           string
	sendChan     chan interface{}
	recvChan     chan *localMsg
	recvBuff     []byte
	sendBuff     []byte
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func NewTunnelConn(id string, sendChan chan interface{}, recvChan chan *localMsg) *TunnelConn {
	return &TunnelConn{
		id: id, sendChan: sendChan, recvChan: recvChan,
		recvBuff: make([]byte, 1024), sendBuff: make([]byte, 1024),
	}
}

func (c *TunnelConn) Read(b []byte) (n int, err error) {
	if len(c.recvBuff) >= len(b) {
		n = len(b)
		copy(b, c.recvBuff)
		c.recvBuff = c.recvBuff[n:]
		return
	}
	if len(c.recvBuff) > 0 {
		copy(b, c.recvBuff)
	}
	n = len(c.recvBuff)

	onRecvData := func(msg *localMsg) {
		data := msg.data.([]byte)
		cn := len(b) - n
		if cn > len(data) {
			cn = len(data)
		}
		copy(b[n:], data[:cn])
		c.recvBuff = data[cn:]
	}

	if c.readTimeout > 0 {
		select {
		case msg := <-c.recvChan:
			onRecvData(msg)
		case <-time.After(c.readTimeout):
			c.recvBuff = c.recvBuff[n:]
			err = errors.New("Read Timeout")
		}
	} else {
		msg := <-c.recvChan
		onRecvData(msg)
	}
	return
}

func (c *TunnelConn) Write(b []byte) (n int, err error) {
	data := make([]byte, len(b), len(b))
	copy(data, b)

	sdata := localMsg{c.id, TCP_SEND, data}

	if c.writeTimeout > 0 {
		select {
		case c.sendChan <- &sdata:
			return len(b), nil
		case <-time.After(c.writeTimeout):
			return 0, errors.New("Write Timeout")
		}
	} else {
		c.sendChan <- &sdata
		return len(b), nil
	}
}

func (c *TunnelConn) LocalAddr() net.Addr {
	return nil
}

func (c *TunnelConn) RemoteAddr() net.Addr {
	return nil
}

func (c *TunnelConn) Close() error {
	close(c.sendChan)
	close(c.recvChan)
	return nil
}

func (c *TunnelConn) SetDeadline(t time.Time) error {
	//c.readTimeout = t
	//c.writeTimeout = t
	return nil
}

func (c *TunnelConn) SetReadDeadline(t time.Time) error {
	//c.readTimeout = t
	return nil
}

func (c *TunnelConn) SetWriteDeadline(t time.Time) error {
	//c.writeTimeout = t
	return nil
}
