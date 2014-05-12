package main

import (
	"errors"
	"io"
	"log"
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

	var msg *localMsg

	if c.readTimeout > 0 {
		select {
		case msg = <-c.recvChan:
		case <-time.After(c.readTimeout):
			c.recvBuff = c.recvBuff[n:]
			err = errors.New("Read Timeout")
			return
		}
	} else {
		msg = <-c.recvChan
	}

	//the channel is closed
	if msg == nil {
		n = 0
		err = errors.New("Closed")
		return
	}

	if msg.msgType == REP_ERROR {
		n = 0
		err = errors.New(string(msg.data.([]byte)))
	} else {
		log.Printf("msg %d , %d, current %d ", msg.msgType, len(msg.data.([]byte)), n)
		data := msg.data.([]byte)
		if len(data) == 0 {
			n = 0
			err = io.EOF
		} else {
			cn := len(b) - n
			if cn > len(data) {
				cn = len(data)
			}
			copy(b[n:n+cn], data[:cn])
			c.recvBuff = data[cn:]
			n = n + cn
		}
	}
	log.Printf("read %d %d", n, len(b))
	return
}

func (c *TunnelConn) Write(b []byte) (n int, err error) {
	data := make([]byte, len(b), len(b))
	copy(data, b)

	sdata := localMsg{c.id, TCP_DATA, data}

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
