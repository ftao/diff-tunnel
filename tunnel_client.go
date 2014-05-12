package main

import (
	"bufio"
	"bytes"
	"code.google.com/p/go-uuid/uuid"
	"errors"
	zmq "github.com/pebbe/zmq4"
	"log"
	"net"
	"net/http"
	"time"
)

type localMsg struct {
	streamId string
	msgType  int
	data     interface{}
}

type TunnelClient struct {
	socket    *zmq.Socket
	respChans map[string]chan *localMsg
	reqChan   chan interface{}
	cache     Cache
}

func NewTunnelClient(remote string) (*TunnelClient, error) {
	//TODO: panic if failed
	socket, _ := zmq.NewSocket(zmq.DEALER)
	socket.Connect(remote)
	return &TunnelClient{
		socket,
		make(map[string]chan *localMsg),
		make(chan interface{}, 1),
		makeCache()}, nil
}

func (c *TunnelClient) Connect(host string) (net.Conn, error) {
	streamId := string([]byte(uuid.NewUUID()))
	respChan := make(chan *localMsg, 1)
	c.respChans[streamId] = respChan

	c.reqChan <- &localMsg{streamId, TCP_CONNECT, host}

	connResp := <-respChan
	if connResp.msgType == REP_ERROR {
		return nil, errors.New("Connect Error : " + connResp.data.(string))
	}
	//TODO: connect timeout

	conn := NewTunnelConn(streamId, c.reqChan, respChan)
	return conn, nil
}

// implement http.RoundTrip interface
// send http request via the tunnel
func (c *TunnelClient) RoundTrip(r *http.Request) (*http.Response, error) {
	reqId := string([]byte(uuid.NewUUID()))
	respChan := make(chan *localMsg, 1)
	c.respChans[reqId] = respChan
	c.reqChan <- &localMsg{streamId: reqId, msgType: HTTP_REQ, data: r}
	//TODO: add timeout , check RFC for status code about proxy timeout
	respMsg := <-respChan
	data := respMsg.data.([]byte)

	reader := bufio.NewReader(bytes.NewBuffer(data))
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		return nil, err
	}
	delete(c.respChans, reqId)
	return resp, nil
}

func (c *TunnelClient) Run() error {
	reactor := zmq.NewReactor()
	reactor.AddSocket(c.socket, zmq.POLLIN, c.onNewResponse)
	reactor.AddChannel(c.reqChan, 1, c.onNewRequest)
	reactor.Run(time.Duration(50) * time.Millisecond)
	return nil
}

func (c *TunnelClient) onNewRequest(msg interface{}) error {
	m := msg.(*localMsg)
	switch m.msgType {
	case HTTP_REQ:
		return c.onNewHttpRequest(m.streamId, m.data.(*http.Request))
	case TCP_CONNECT:
		return c.onNewTcpConnect(m.streamId, m.data.(string))
	}
	return errors.New("invalid request")
}

func (c *TunnelClient) onNewHttpRequest(reqId string, request *http.Request) error {

	var buff bytes.Buffer
	request.WriteProxy(&buff)

	cacheKey := makeCacheKey(request)
	rh := RequestHeader{Action: HTTP_REQ, CacheKey: cacheKey}

	if cacheItem, ok := c.cache.Get(cacheKey); ok {
		rh.Version = cacheItem.Version
	}

	header, _ := MarshalBinary(&rh)
	body := buff.Bytes()

	log.Print("send req header ", string(header))

	c.socket.Send(reqId, zmq.SNDMORE)
	c.socket.Send("", zmq.SNDMORE)
	c.socket.SendBytes(header, zmq.SNDMORE) //local cache version
	c.socket.SendBytes(body, 0)             //send actuall request

	return nil
}

func (c *TunnelClient) onNewTcpConnect(streamId string, host string) error {
	rh := RequestHeader{Action: TCP_CONNECT}
	header, _ := MarshalBinary(&rh)

	c.socket.Send(streamId, zmq.SNDMORE)
	c.socket.Send("", zmq.SNDMORE)
	c.socket.SendBytes(header, zmq.SNDMORE) //local cache version
	c.socket.Send(host, 0)                  //send actuall request

	return nil
}

func (c *TunnelClient) onNewResponse(state zmq.State) (err error) {
	msgs, err := c.socket.RecvMessageBytes(0)
	if err != nil {
		return
	}
	if len(msgs) != 4 {
		return errors.New("unexpected msg count")
	}
	reqId := string(msgs[0])
	//msgs[1] zero size

	var ph ResponseHeader
	_ = UnmarshalBinary(&ph, msgs[2])

	log.Printf("recv resp header %s , body length %s", msgs[2], len(msgs[3]))

	respBytes := msgs[3]
	if ph.Action == HTTP_DIFF_REP && ph.IsPatch {
		cacheItem, ok := c.cache.Get(ph.CacheKey)
		if ok && bytes.Equal(cacheItem.Version, ph.PatchTo) {
			respBytes = Patch(cacheItem.Value, respBytes)
		}
	}

	//update cache
	c.cache.Set(ph.CacheKey, &CacheItem{ph.Version, respBytes})

	respChan, ok := c.respChans[reqId]
	if !ok {
		return errors.New("invalid request id")
	}

	respChan <- &localMsg{streamId: reqId, msgType: HTTP_REP, data: respBytes}

	return nil
}

func (c *TunnelClient) Close() error {
	c.socket.Close()
	return nil
}
