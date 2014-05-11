package main

import (
	"bufio"
	"bytes"
	"errors"
	zmq "github.com/pebbe/zmq4"
	"log"
	"net/http"
	"time"
)

type response struct {
	connId    string
	reqId     string
	reqHeader *RequestHeader
	resp      *http.Response
	err       error
}

type TunnelServer struct {
	socket   *zmq.Socket
	respChan chan interface{}
	rt       http.RoundTripper
	cache    Cache
}

func NewTunnelServer(bind string) (*TunnelServer, error) {
	socket, _ := zmq.NewSocket(zmq.ROUTER)
	socket.Bind(bind)
	return &TunnelServer{socket,
		make(chan interface{}),
		&http.Transport{},
		makeCache()}, nil
}

func (s *TunnelServer) Run() error {
	reactor := zmq.NewReactor()
	reactor.AddSocket(s.socket, zmq.POLLIN, s.onNewRequest)
	reactor.AddChannel(s.respChan, 1, s.onNewResponse)
	reactor.Run(time.Duration(50) * time.Millisecond)
	return nil
}

func (s *TunnelServer) send(envelope []string, header []byte, body []byte) (err error) {
	for _, item := range envelope {
		s.socket.Send(item, zmq.SNDMORE)
	}
	log.Print("send resp header ", string(header))
	s.socket.SendBytes(header, zmq.SNDMORE)
	s.socket.SendBytes(body, 0)

	return nil
}

func (s *TunnelServer) onNewResponse(item interface{}) (err error) {
	r := item.(*response)

	envelope := []string{
		r.connId,
		r.reqId,
		"",
	}

	var header []byte
	var body []byte

	rh := r.reqHeader
	if r.err != nil {
		ph := ResponseHeader{Action: RESP_ERROR}
		header, _ = MarshalBinary(&ph)
		s.send(envelope, header, []byte(r.err.Error()))
		return nil
	}

	var buff bytes.Buffer
	r.resp.Write(&buff)

	nk := s.cache.GenVersion(rh.CacheKey)
	ph := ResponseHeader{CacheKey: rh.CacheKey, Version: nk}

	//check for cache , if possible make diff, and send back diff
	cacheItem, ok := s.cache.Get(rh.CacheKey)
	if ok {
		log.Printf("Hit Cache : %s , %s,  %s", rh.CacheKey, rh.Version, cacheItem.Version)
	}
	if ok && bytes.Equal(cacheItem.Version, rh.Version) {
		log.Printf("Hit Cache : %s , %s", rh.CacheKey, rh.Version)
		ph.Action = RESP_DIFF
		ph.IsPatch = true
		ph.PatchTo = rh.Version

		if bytes.Equal(cacheItem.Value, buff.Bytes()) {
			ph.Version = rh.Version
		} else {
			s.cache.Set(rh.CacheKey, &CacheItem{nk, buff.Bytes()})
		}
		body = MakeDiff(cacheItem.Value, buff.Bytes())
	} else {
		ph.Action = RESP_HTTP
		body = buff.Bytes()
		s.cache.Set(rh.CacheKey, &CacheItem{nk, buff.Bytes()})
		log.Printf("Set Cache : %s , %s", rh.CacheKey, nk)
	}

	header, _ = MarshalBinary(&ph)
	s.send(envelope, header, body)

	return nil

}

func (s *TunnelServer) onNewRequest(state zmq.State) (err error) {
	msgs, err := s.socket.RecvMessage(0)
	if err != nil {
		return
	}
	if len(msgs) != 5 {
		return errors.New("unexpected msg count")
	}

	var rh RequestHeader
	_ = UnmarshalBinary(&rh, []byte(msgs[3]))

	log.Print("recv req header ", msgs[3])

	go func() {
		reader := bufio.NewReader(bytes.NewBufferString(msgs[4]))
		req, _ := http.ReadRequest(reader)
		resp, err := s.rt.RoundTrip(req)
		if err != nil {
			s.respChan <- &response{msgs[0], msgs[1], &rh, nil, err}
		} else {
			s.respChan <- &response{msgs[0], msgs[1], &rh, resp, nil}
		}
	}()
	return nil
}

func (s *TunnelServer) Close() error {
	s.socket.Close()
	zmq.Term()
	return nil
}
