package main

import (
	"io"
)

type BytesCarrier interface {
	GetBytes() []byte
}

type ChannelReaderCloser struct {
	channel chan interface{}
	buff    []byte
}

func (c *ChannelReaderCloser) readFromChannel(p []byte) (n int, err error) {
	n = 0
	msg := <-c.channel
	if msg == nil {
		err = io.EOF
		return
	}

	var data []byte
	switch msg := msg.(type) {
	case []byte:
		data = msg
	case string:
		data = []byte(msg)
	case BytesCarrier:
		data = msg.GetBytes()
	default:
		//print error
	}

	n = len(p)
	if len(p) >= len(data) {
		n = len(data)
	}
	if n > 0 {
		copy(p, data[:n])
	}

	c.buff = data[n:]

	return
}

func (c *ChannelReaderCloser) Read(p []byte) (n int, err error) {
	//first read from buff
	n = len(c.buff)
	//if buff is bigger thant needed
	if len(c.buff) >= len(p) {
		n = len(p)
		copy(p, c.buff[:n])
		c.buff = c.buff[n:]
		return
	}

	if n > 0 {
		copy(p, c.buff)
	}

	var cn int
	cn, err = c.readFromChannel(p[n:])
	n = n + cn
	return
}

func (c *ChannelReaderCloser) Close() error {
	return nil
}

type ChannelWriterCloser struct {
	channel chan interface{}
	prefix  [][]byte
}

func (c *ChannelWriterCloser) Write(p []byte) (n int, err error) {
	msg := make([][]byte, len(c.prefix)+1)
	copy(msg, c.prefix)
	msg[len(c.prefix)] = p

	c.channel <- msg
	n = len(p)
	return
}

func (c *ChannelWriterCloser) Close() error {
	close(c.channel)
	return nil
}
