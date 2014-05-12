package main

import (
	"errors"
	"io"
	"log"
	"net"
)

type TcpWorker struct {
	reqChan     chan interface{}
	repChan     chan interface{}
	connections map[string]net.Conn
}

func NewTcpWorker(repChan chan interface{}) *TcpWorker {
	reqChan := make(chan interface{}, 1)
	connections := make(map[string]net.Conn)
	return &TcpWorker{reqChan, repChan, connections}
}

func (tw *TcpWorker) Run() {
	for {
		req := <-tw.reqChan
		go func() {
			rep := tw.handle(req.([][]byte))
			if len(rep) > 0 {
				tw.repChan <- rep
			}
		}()
	}
}

func (tw *TcpWorker) SendRequest(req [][]byte) {
	tw.reqChan <- req
}

func (tw *TcpWorker) readTo(conn net.Conn, envelope [][]byte) {
	ph := ResponseHeader{Action: TCP_DATA}
	buff := make([]byte, 5*1024)
	for {
		n, err := conn.Read(buff)
		if n > 0 {
			body := make([]byte, n)
			copy(body, buff[:n])
			tw.repChan <- tw.makeRep(envelope, &ph, body)
		}
		if err == io.EOF {
			tw.repChan <- tw.makeRep(envelope, &ph, []byte(""))
		} else if err != nil {
			tw.repChan <- tw.makeErrorRep(envelope, err)
		}
	}
}

func (tw *TcpWorker) handleConnect(req [][]byte) [][]byte {
	var ph ResponseHeader

	conn, err := net.Dial("tcp", string(req[4]))
	if err != nil {
		return tw.makeErrorRep(req[:3], err)
	} else {
		ph = ResponseHeader{Action: TCP_CONNECT_REP}
		tw.connections[string(req[1])] = conn
		go tw.readTo(conn, req[0:3])
		return tw.makeRep(req[:3], &ph, []byte(""))
	}
}

func (tw *TcpWorker) handleData(req [][]byte) [][]byte {
	conn, ok := tw.connections[string(req[1])]
	if !ok {
		log.Printf("connection not found : %s", req[1])
	}
	body := req[4]
	pos := 0
	for {
		n, err := conn.Write(body)
		if err != nil {
			log.Printf("Write Error")
			return tw.makeErrorRep(req[:3], errors.New("Write Error"))
		}
		pos += n
		if pos >= len(body) {
			break
		}
	}
	return [][]byte{}
}

func (tw *TcpWorker) handle(req [][]byte) [][]byte {
	var rh RequestHeader
	_ = UnmarshalBinary(&rh, []byte(req[3]))
	switch rh.Action {
	case TCP_CONNECT:
		return tw.handleConnect(req)
	case TCP_DATA:
		return tw.handleData(req)
	}
	return [][]byte{}
}

func (tw *TcpWorker) makeRep(envelope [][]byte, ph *ResponseHeader, body []byte) [][]byte {
	elen := len(envelope)
	rep := make([][]byte, elen+2)
	copy(rep[:len(envelope)], envelope)
	header, _ := MarshalBinary(ph)
	rep[elen] = header
	rep[elen+1] = body
	return rep
}

func (tw *TcpWorker) makeErrorRep(envelope [][]byte, err error) [][]byte {
	ph := ResponseHeader{Action: REP_ERROR}
	return tw.makeRep(envelope, &ph, []byte(err.Error()))
}
