package main

import (
	"bufio"
	"bytes"
	"net/http"
	"testing"
)

func TestTcpWorker(t *testing.T) {
	//curl -X POST -d @README.md -H "Content-Type: text/plain" http://httpbin.org/post -vvvv
	reqLines := []string{
		"GET /get HTTP/1.1\r\n",
		"User-Agent: curl/7.22.0 (x86_64-pc-linux-gnu) libcurl/7.22.0 OpenSSL/1.0.1 zlib/1.2.3.4 libidn/1.23 librtmp/2.3\r\n",
		"Host: httpbin.org\r\n",
		"Accept: */*\r\n",
		"\r\n",
	}

	reqChan := make(chan *Msg)
	repChan := make(chan *Msg)

	worker := &TcpWorker{reqChan}

	host := "httpbin.org:80"
	sid := MakeUID()
	flag := FLAG_TCP
	body := []byte(host)
	msgType := TCP_CONNECT
	ct := CT_RAW
	connectMsg := &Msg{
		Header: &Header{StreamId: sid, Version: VERSION_1, Flag: flag | FLAG_STREAM_BEGIN, MsgType: msgType},
		Body:   &TcpData{ContentType: ct, Payload: body},
	}

	go worker.Run(repChan)

	reqChan <- connectMsg

	connectRep := <-repChan

	if connectRep.GetMsgType() != TCP_CONNECT_REP {
		t.Errorf("fail to connect %s", host)
	}

	for _, line := range reqLines {
		reqChan <- &Msg{
			Header: &Header{StreamId: sid, Version: VERSION_1, Flag: flag, MsgType: TCP_DATA},
			Body:   &TcpData{ContentType: ct, Payload: []byte(line)},
		}
	}

	ret := make([]byte, 0, 10*1024)
	for {
		msg := <-repChan
		ret = append(ret, msg.Body.(*TcpData).GetPayload()...)

		_, err := http.ReadResponse(bufio.NewReader(bytes.NewBuffer(ret)), nil)
		if err == nil {
			break
		}
	}

	if !bytes.HasPrefix(ret, []byte("HTTP/1.1 200 OK\r\n")) {
		t.Errorf("expect 200, got %s", string(ret))
	}

	// close the connection
	reqChan <- &Msg{
		Header: &Header{MsgType: TCP_DATA, StreamId: sid, Version: VERSION_1, Flag: flag},
		Body:   &TcpData{ContentType: ct, Payload: []byte("")},
	}

}
