package main

import (
	"bytes"
	"io"
	"testing"
)

func TestTunnelWriter(t *testing.T) {
	data := [][]byte{
		[]byte("GET /hello/ HTTP 1.1\r\n"),
		[]byte("Host: example.com\r\n"),
		[]byte("\r\n"),
	}

	sendChan := make(chan *Msg)
	writer := &TunnelWriter{
		sendChan: sendChan,
		msgMaker: &msgBuilder{defaultFlags: FLAG_TCP},
	}

	go func() {
		for _, item := range data {
			writer.Write(item)
		}
		writer.Close()
	}()

	var i int
	send := make([]byte, 0)
	for msg := range sendChan {
		i += 1
		if msg.GetMsgType() != TCP_DATA {
			t.Errorf("msg type not right got %v", msg)
		}
		send = append(send, msg.Body.(*TcpData).GetPayload()...)
		if msg.IsEndOfStream() {
			break
		}
	}

	alldata := bytes.Join(data, []byte(""))
	if !bytes.Equal(send, alldata) {
		t.Errorf("write result length match input, expect %v, got %v", len(alldata), len(send))
		t.Errorf("write result not match input, expect %v, got %v", string(alldata), string(send))
	}
}

func TestTunnelReader(t *testing.T) {
	sid := MakeUID()
	envelope := [][]byte{[]byte("")}
	flag := FLAG_TCP

	data := [][]byte{
		[]byte("GET /hello/ HTTP 1.1\r\n"),
		[]byte("Host: example.com\r\n"),
		[]byte("\r\n"),
	}

	msgs := []*Msg{
		&Msg{
			Envelope: envelope,
			Header:   &Header{MsgType: TCP_DATA, StreamId: sid, Version: VERSION_1, Flag: flag | FLAG_STREAM_BEGIN},
			Body:     &TcpData{ContentType: CT_RAW, Payload: data[0]},
		},
		&Msg{
			Envelope: envelope,
			Header:   &Header{MsgType: TCP_DATA, StreamId: sid, Version: VERSION_1, Flag: flag},
			Body:     &TcpData{ContentType: CT_RAW, Payload: data[1]},
		},
		&Msg{
			Envelope: envelope,
			Header:   &Header{MsgType: TCP_DATA, StreamId: sid, Version: VERSION_1, Flag: flag},
			Body:     &TcpData{ContentType: CT_RAW, Payload: data[2]},
		},
		&Msg{
			Envelope: envelope,
			Header:   &Header{MsgType: TCP_DATA, StreamId: sid, Version: VERSION_1, Flag: flag | FLAG_STREAM_END},
			Body:     &TcpData{ContentType: CT_RAW, Payload: []byte("")},
		},
	}

	readChan := make(chan *Msg)
	reader := &TunnelReader{recvChan: readChan}

	go func() {
		for _, msg := range msgs {
			readChan <- msg
		}
	}()
	var n int
	var err error
	ret := make([]byte, 0)
	buff := make([]byte, 1024, 1024)
	//read all
	for {
		n, err = reader.Read(buff)
		ret = append(ret, buff[:n]...)
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			break
		}
	}
	alldata := bytes.Join(data, []byte(""))
	if !bytes.Equal(ret, alldata) {
		t.Errorf("read result length match input, expect %v, got %v", len(alldata), len(ret))
		t.Errorf("read result not match input, expect %v, got %v", string(alldata), string(ret))
	}
}

func TestTunnelReaderBigMessage(t *testing.T) {
	sid := MakeUID()
	envelope := [][]byte{[]byte("")}
	flag := FLAG_TCP

	data := make([]byte, 4096*5)
	copy(data[0:5], []byte("hello"))
	data[4096*5-2] = '\r'
	data[4096*5-2] = '\n'

	msg := &Msg{
		Envelope: envelope,
		Header:   &Header{MsgType: TCP_DATA, StreamId: sid, Version: VERSION_1, Flag: flag | FLAG_STREAM_BEGIN | FLAG_STREAM_END},
		Body:     &TcpData{ContentType: CT_RAW, Payload: data},
	}

	readChan := make(chan *Msg)
	reader := &TunnelReader{recvChan: readChan}
	go func() {
		readChan <- msg
	}()

	var n int
	var err error
	ret := make([]byte, 0)
	buff := make([]byte, 4096, 4096)

	for {
		n, err = reader.Read(buff)
		ret = append(ret, buff[:n]...)
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			break
		}
	}

	if !bytes.Equal(ret, data) {
		t.Errorf("read result length not match input, expect %v, got %v", len(data), len(ret))
		//	t.Errorf("read result not match input, expect %x, got %x", data, ret)
	}

}
