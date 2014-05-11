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

type response struct {
	connId    string
	reqId     string
	reqHeader *RequestHeader
	resp      *http.Response
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

func (s *TunnelServer) onNewResponse(item interface{}) (err error) {
	r := item.(*response)

	s.socket.Send(r.connId, zmq.SNDMORE)
	s.socket.Send(r.reqId, zmq.SNDMORE)
	s.socket.Send("", zmq.SNDMORE)

	var buff bytes.Buffer
	r.resp.Write(&buff)

	rh := r.reqHeader
	nk := uuid.New()
	var ph ResponseHeader

	//check for cache , if possible make diff, and send back diff
	cacheItem, ok := s.cache.Get(rh.CacheKey)
	if ok && (cacheItem.Version == rh.Version) {
		log.Printf("Hit Cache : %s , %s", rh.CacheKey, rh.Version)
		ph = ResponseHeader{RESP_DIFF, rh.CacheKey, true, nk, rh.Version}
		if bytes.Equal(cacheItem.Value, buff.Bytes()) {
			ph.Version = rh.Version
		} else {
			s.cache.Set(rh.CacheKey, &CacheItem{nk, buff.Bytes()})
		}
		phBytes, _ := json.Marshal(&ph)
		log.Print("send resp header ", string(phBytes))
		diff := MakeDiff(cacheItem.Value, buff.Bytes())
		s.socket.SendBytes(phBytes, zmq.SNDMORE) //used cache version
		s.socket.SendBytes(diff, 0)              //send actuall diff
	} else {
		ph = ResponseHeader{RESP_HTTP, rh.CacheKey, false, nk, ""}
		phBytes, _ := json.Marshal(&ph)
		log.Print("send resp header ", string(phBytes))
		s.socket.SendBytes(phBytes, zmq.SNDMORE) //used cache version
		s.socket.SendBytes(buff.Bytes(), 0)      //send actuall diff
		s.cache.Set(rh.CacheKey, &CacheItem{nk, buff.Bytes()})
	}

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
	json.Unmarshal([]byte(msgs[3]), &rh)
	//TODO: chek action

	log.Print("recv req header ", msgs[3])

	go func() {
		reader := bufio.NewReader(bytes.NewBufferString(msgs[4]))
		req, _ := http.ReadRequest(reader)
		resp, err := s.rt.RoundTrip(req)
		if err != nil {
			log.Print("fail to fetch error: ", err)
		}
		s.respChan <- &response{msgs[0], msgs[1], &rh, resp}
	}()
	return nil
}

func (s *TunnelServer) Close() error {
	s.socket.Close()
	zmq.Term()
	return nil
}
