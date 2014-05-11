package main

import (
	"bufio"
	"bytes"
	"code.google.com/p/go-uuid/uuid"
	"encoding/json"
	"errors"
	zmq "github.com/pebbe/zmq4"
	"log"
	"net/http"
	"time"
)

type request struct {
	id      string
	request *http.Request
}

type TunnelClient struct {
	remote     string
	socket     *zmq.Socket
	requestMap map[string]chan *http.Response
	reqChan    chan interface{}
	cache      Cache
}

func NewTunnelClient(remote string) (*TunnelClient, error) {
	socket, _ := zmq.NewSocket(zmq.DEALER)
	socket.Connect(remote)
	return &TunnelClient{remote, socket,
		make(map[string]chan *http.Response),
		make(chan interface{}),
		makeCache()}, nil
}

func (c *TunnelClient) Do(r *http.Request) (*http.Response, error) {
	reqId := uuid.New()
	respChan := make(chan *http.Response)
	c.requestMap[reqId] = respChan
	c.reqChan <- &request{reqId, r}
	resp := <-respChan
	delete(c.requestMap, reqId)
	return resp, nil
}

func (c *TunnelClient) handleRequest(r *request) error {
	var buff bytes.Buffer
	r.request.WriteProxy(&buff)
	c.socket.Send(r.id, zmq.SNDMORE)
	c.socket.Send("", zmq.SNDMORE)

	cacheKey := makeCacheKey(r.request)
	rh := RequestHeader{REQ_HTTP_GET, cacheKey, ""}

	if cacheItem, ok := c.cache.Get(cacheKey); ok {
		rh.Version = cacheItem.Version
	}

	rhBytes, _ := json.Marshal(&rh)
	log.Print("send req header ", string(rhBytes))
	c.socket.SendBytes(rhBytes, zmq.SNDMORE) //local cache version
	c.socket.SendBytes(buff.Bytes(), 0)      //send actuall request
	return nil
}

func (c *TunnelClient) Run() error {
	reactor := zmq.NewReactor()
	reactor.AddSocket(c.socket, zmq.POLLIN, c.onNewResponse)
	reactor.AddChannel(c.reqChan, 1, c.onNewRequest)
	reactor.Run(time.Duration(50) * time.Millisecond)
	return nil
}

func (c *TunnelClient) onNewRequest(req interface{}) error {
	return c.handleRequest(req.(*request))
}

func (c *TunnelClient) onNewResponse(state zmq.State) (err error) {
	msgs, err := c.socket.RecvMessage(0)
	if err != nil {
		return
	}
	if len(msgs) != 4 {
		return errors.New("unexpected msg count")
	}
	reqId := msgs[0]
	//msgs[1] zero size

	var ph ResponseHeader
	json.Unmarshal([]byte(msgs[2]), &ph)

	log.Print("recv resp header ", msgs[2])
	log.Print("recv resp body length ", len(msgs[3]))

	respBytes := []byte(msgs[3])
	if ph.Action == RESP_DIFF && ph.IsPatch {
		cacheItem, ok := c.cache.Get(ph.CacheKey)
		if ok && cacheItem.Version == ph.PatchTo {
			respBytes = Patch(cacheItem.Value, respBytes)
		}
	}

	//update cache
	c.cache.Set(ph.CacheKey, &CacheItem{ph.Version, respBytes})

	reader := bufio.NewReader(bytes.NewBuffer(respBytes))
	resp, err := http.ReadResponse(reader, nil)
	respChan, ok := c.requestMap[reqId]
	if !ok {
		return errors.New("invalid request id")
	}
	respChan <- resp
	return nil
}

func (c *TunnelClient) Close() error {
	c.socket.Close()
	zmq.Term()
	return nil
}
