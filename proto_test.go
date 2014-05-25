package dtunnel

import (
	"bytes"
	"reflect"
	"testing"
)

func TestMsgEncodeDecode(t *testing.T) {
	sid := MakeUID()
	flag := FLAG_TCP | FLAG_STREAM_BEGIN
	body := []byte("www.google.com:443")
	msgType := TCP_CONNECT
	ct := CT_RAW
	connectMsg := &Msg{
		Envelope: [][]byte{[]byte("")},
		Header:   &Header{MsgType: msgType, StreamId: sid, Version: VERSION_1, Flag: flag},
		Body:     &TcpData{ContentType: ct, Payload: body},
	}

	frames, _ := toFrames(connectMsg)

	newMsg, err := fromFrames(frames)
	if err != nil {
		t.Errorf("fromFrames , err should not be nil: %v", err)
	}

	newFrames, _ := toFrames(newMsg)

	if !reflect.DeepEqual(connectMsg, newMsg) {
		t.Error("old msg & new msg should be equal")
	}

	if !reflect.DeepEqual(frames, newFrames) {
		t.Error("old frame & new frames should be equal")
	}

	if newMsg.GetMsgType() != msgType {
		t.Errorf("msg type not match expecet:%s got:%s", msgType, newMsg.GetMsgType)
	}

	if payload := newMsg.Body.(*TcpData).GetPayload(); !bytes.Equal(payload, body) {
		t.Errorf("msg body not match expecet:%s got:%s", body, payload)
	}

	if !newMsg.TestFlag(FLAG_TCP) {
		t.Error("msg should have FLAG_TCP")
	}

	if !newMsg.TestFlag(FLAG_STREAM_BEGIN) {
		t.Error("msg should have FLAG_TCP")
	}
}
