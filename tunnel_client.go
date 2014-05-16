package main

import (
	"bytes"
	"fmt"
	zmq "github.com/pebbe/zmq4"
	"io"
	"log"
	"net"
	"net/http"
)

type TunnelClient struct {
	socket   *zmq.Socket
	repChans map[UID]chan *Msg
	reqChan  chan *Msg
	cm       *CacheManager
}

func NewTunnelClient(remote string) (*TunnelClient, error) {
	//TODO: panic if failed
	socket, _ := zmq.NewSocket(zmq.DEALER)
	socket.Connect(remote)
	return &TunnelClient{
		socket,
		make(map[UID]chan *Msg),
		make(chan *Msg, 1),
		makeCacheManager(),
	}, nil
}

func (c *TunnelClient) ConnectTcp(host string) (net.Conn, error) {
	sid := MakeUID()
	repChan := make(chan *Msg, 1)
	c.repChans[sid] = repChan

	c.reqChan <- makeReqMsg(sid, TCP_CONNECT, CT_RAW, []byte(host), FLAG_TCP|FLAG_STREAM_BEGIN)

	msg := <-repChan

	envelope := [][]byte{[]byte("")}
	if msg.GetMsgType() == ERROR {
		return nil, fmt.Errorf("Connect Error : %s", msg.Body)
	}
	conn := &TunnelConn{
		&TunnelReader{recvChan: repChan},
		&TunnelWriter{
			sendChan: c.reqChan,
			msgMaker: NewMsgBuilder(sid, envelope, FLAG_TCP),
		},
	}
	return conn, nil
}

// implement http.RoundTrip interface
// send http request via the tunnel
func (c *TunnelClient) RoundTrip(r *http.Request) (io.ReadCloser, error) {
	sid := MakeUID()
	repChan := make(chan *Msg, 1)
	c.repChans[sid] = repChan
	envelope := [][]byte{[]byte("")}
	var reader io.ReadCloser
	var writer io.WriteCloser

	cacheKey := makeCacheKey(r)
	reader = &CachedTunnelReader{
		&TunnelReader{recvChan: repChan, cache: c.cm.local},
		c.cm.local,
		cacheKey,
		new(bytes.Buffer),
	}
	writer = &TunnelWriter{
		sendChan: c.reqChan,
		msgMaker: NewMsgBuilder(sid, envelope, FLAG_HTTP|FLAG_TCP),
	}

	if c.cm != nil {
		cacheKey := makeCacheKey(r)
		digest, ok := c.cm.local.GetDigest(cacheKey)
		if ok {
			c.reqChan <- makeCacheShareMsg(cacheKey, digest)
		}
	}

	go func() {
		r.WriteProxy(writer)
		writer.Close()
	}()

	return reader, nil
}

func (c *TunnelClient) Run() error {

	//just to solve zmq socket thread safe problem
	go func() {
		for msg := range c.reqChan {
			frames, err := toFrames(msg)
			if err != nil {
				log.Printf("fail to build frames: %s", err.Error())
				continue
			}
			log.Printf("[tc]send msg %s", msg)
			c.socket.SendMessage(frames)
		}
		log.Print("reach end of reqChan, should not happen")
	}()

	for {
		frames, err := c.socket.RecvMessageBytes(0)
		if err != nil {
			log.Printf("[tc]recv zmq error %s", err)
			continue
		}
		msg, err := fromFrames(frames)
		if err != nil {
			log.Printf("invalid frames : %s", err.Error())
			continue
		}

		log.Printf("[tc]recv msg %s", msg)
		sid := msg.GetStreamId()
		repChan, ok := c.repChans[sid]
		if !ok {
			log.Printf("invalid request id: %s", sid)
			continue
		}
		repChan <- msg
		if msg.IsEndOfStream() {
			close(repChan)
		}
	}
	log.Print("should never reach here")
	return nil
}

func (c *TunnelClient) Close() error {
	c.socket.Close()
	return nil
}
