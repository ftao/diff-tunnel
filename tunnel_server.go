package main

import (
	"errors"
	zmq "github.com/pebbe/zmq4"
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
	socket     *zmq.Socket
	repChan    chan interface{}
	httpWorker *HttpWorker
	tcpWorker  *TcpWorker
}

func NewTunnelServer(bind string) (*TunnelServer, error) {
	socket, _ := zmq.NewSocket(zmq.ROUTER)
	socket.Bind(bind)
	repChan := make(chan interface{}, 1)
	return &TunnelServer{socket, repChan, NewHttpWorker(repChan), NewTcpWorker(repChan)}, nil
}

func (s *TunnelServer) Run() error {
	go s.httpWorker.Run()
	go s.tcpWorker.Run()
	reactor := zmq.NewReactor()
	reactor.AddSocket(s.socket, zmq.POLLIN, s.onNewRequest)
	reactor.AddChannel(s.repChan, 1, s.onNewResponse)
	reactor.Run(time.Duration(50) * time.Millisecond)
	return nil
}

func (s *TunnelServer) onNewResponse(msgs interface{}) (err error) {
	s.socket.SendMessage(msgs)
	return nil
}

func (s *TunnelServer) onNewRequest(state zmq.State) (err error) {
	msgs, err := s.socket.RecvMessageBytes(0)
	if err != nil {
		return
	}
	if len(msgs) != 5 {
		return errors.New("unexpected msg count")
	}

	var rh RequestHeader
	_ = UnmarshalBinary(&rh, []byte(msgs[3]))

	switch rh.Action {
	case HTTP_REQ:
		s.httpWorker.SendRequest(msgs)
	case TCP_CONNECT:
		s.tcpWorker.SendRequest(msgs)
	case TCP_DATA:
		s.tcpWorker.SendRequest(msgs)
	}

	return nil
}

func (s *TunnelServer) Close() error {
	s.socket.Close()
	return nil
}
