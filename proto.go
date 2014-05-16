package main

import (
	"bytes"
	"code.google.com/p/go-uuid/uuid"
	"encoding"
	"encoding/binary"
	"errors"
	"fmt"
	_ "log"
	"strings"
)

var InvalidFrameCount error = errors.New("Invalid Frame Count")
var InvalidHeader error = errors.New("Invalid Header")
var InvalidBody error = errors.New("Invalid Body")

const (
	INVALID uint16 = 0

	TCP_CONNECT     uint16 = 1
	TCP_CONNECT_REP uint16 = 2
	TCP_DATA        uint16 = 3

	HTTP_CONNECT uint16 = 11
	HTTP_DATA    uint16 = 12

	CACHE_SHARE uint16 = 21

	//CACHE_SHARE uint16 = 51
	ERROR uint16 = 255
)

var TYPE_NAMES map[uint16]string = map[uint16]string{
	TCP_CONNECT:     "TCP_CONNECT",
	TCP_CONNECT_REP: "TCP_CONNECT_REP",
	TCP_DATA:        "TCP_DATA",
	CACHE_SHARE:     "CACHE_SHARE",
	ERROR:           "ERROR",
}

const (
	VERSION_1 uint8 = 1
)

const (
	FLAG_TCP          uint16 = 1
	FLAG_UDP          uint16 = 2
	FLAG_HTTP         uint16 = 4
	FLAG_STREAM_BEGIN uint16 = 8
	FLAG_STREAM_END   uint16 = 16
)

var FLAG_NAMES map[uint16]string = map[uint16]string{
	FLAG_TCP:          "FLAG_TCP",
	FLAG_UDP:          "FLAG_UDP",
	FLAG_HTTP:         "FLAG_HTTP",
	FLAG_STREAM_BEGIN: "FLAG_STREAM_BEGIN",
	FLAG_STREAM_END:   "FLAG_STREAM_END",
}

const (
	CT_RAW        uint16 = 0
	CT_CACHE_DIFF uint16 = 1
)

var CT_NAMES map[uint16]string = map[uint16]string{
	CT_RAW:        "CT_RAW",
	CT_CACHE_DIFF: "CT_CACHE_DIFF",
}

type UID [12]byte

func MakeUID() UID {
	var uid UID
	copy(uid[:], []byte(uuid.NewUUID())[:12])
	return uid
}

const (
	HEADER_SIZE = 18
)

//20 bytes
type Header struct {
	Version  uint8
	StreamId UID
	MsgType  uint16
	Flag     uint16
	Reserved uint8
}

func (h *Header) GetStreamId() UID {
	return h.StreamId
}

func (h *Header) SetStreamId(sid UID) {
	h.StreamId = sid
}

func (h *Header) TestFlag(flag uint16) bool {
	return (h.Flag & flag) > 0
}

func (h *Header) SetFlag(flag uint16) {
	h.Flag = flag
}

func (h *Header) AddFlag(flag uint16) {
	h.Flag = h.Flag | flag
}

func (h *Header) IsEndOfStream() bool {
	return h.TestFlag(FLAG_STREAM_END)
}

func (h *Header) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, HEADER_SIZE)
	buff := bytes.NewBuffer(data)
	err := binary.Write(buff, binary.BigEndian, h)
	return buff.Bytes(), err
}

func (h *Header) UnmarshalBinary(data []byte) error {
	if len(data) != HEADER_SIZE {
		return fmt.Errorf("invalid data length, should be %d, got %d", HEADER_SIZE, len(data))
	}
	buff := bytes.NewBuffer(data)
	return binary.Read(buff, binary.BigEndian, h)
}

type Body interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
	String() string
}

type ErrorData struct {
	ContentType uint16
	Payload     []byte
}

func (d *ErrorData) String() string {
	return string(d.Payload)
}

func (d *ErrorData) MarshalBinary() (b []byte, err error) {
	buff := new(bytes.Buffer)
	binary.Write(buff, binary.BigEndian, d.ContentType)
	binary.Write(buff, binary.BigEndian, d.Payload)
	return buff.Bytes(), nil
}

func (d *ErrorData) UnmarshalBinary(b []byte) (err error) {
	if len(b) < 2 {
		return errors.New("invalid data length, at least 2")
	}
	buff := bytes.NewBuffer(b)
	binary.Read(buff, binary.BigEndian, &d.ContentType)
	binary.Read(buff, binary.BigEndian, &d.Payload)
	return
}

type TcpData struct {
	ContentType uint16
	Payload     []byte
}

func (td *TcpData) MarshalBinary() (b []byte, err error) {
	buff := new(bytes.Buffer)
	binary.Write(buff, binary.BigEndian, td.ContentType)
	binary.Write(buff, binary.BigEndian, td.Payload)
	return buff.Bytes(), nil
}

func (td *TcpData) UnmarshalBinary(b []byte) (err error) {
	if len(b) < 2 {
		return errors.New("invalid data length, at least 2")
	}
	buff := bytes.NewBuffer(b)
	binary.Read(buff, binary.BigEndian, &td.ContentType)
	td.Payload = make([]byte, len(b)-2)
	binary.Read(buff, binary.BigEndian, td.Payload)

	return nil
}

func (td *TcpData) GetPayload() []byte {
	return td.Payload
}

func (td *TcpData) String() string {
	return fmt.Sprintf("content-type=%s size=%d", CT_NAMES[td.ContentType], len(td.Payload))
}

type Msg struct {
	Envelope [][]byte
	*Header
	Body
}

func (m *Msg) GetMsgType() uint16 {
	return m.Header.MsgType
}

func (m *Msg) GetMsgTypeName() string {
	return TYPE_NAMES[m.Header.MsgType]
}

func (m *Msg) GetFlagNames() []string {
	flags := make([]string, 0)
	for k, v := range FLAG_NAMES {
		if m.TestFlag(k) {
			flags = append(flags, v)
		}
	}
	return flags
}

func (m *Msg) String() string {
	return fmt.Sprintf(
		"[%x] <%s> flags=%s %s",
		m.GetStreamId(),
		m.GetMsgTypeName(),
		strings.Join(m.GetFlagNames(), "|"),
		m.Body,
	)
}

func fromFrames(data [][]byte) (m *Msg, err error) {
	var headerPos int = 0
	for i, part := range data {
		if len(part) == 0 {
			headerPos = i + 1
			break
		}
	}
	if len(data) != headerPos+2 {
		return nil, InvalidFrameCount
	}

	var header *Header = new(Header)
	err = header.UnmarshalBinary(data[headerPos])
	if err != nil {
		return nil, err //InvalidHeader
	}

	var body Body

	switch header.MsgType {
	case CACHE_SHARE:
		body = new(CacheShareData)
	case TCP_CONNECT, TCP_DATA, TCP_CONNECT_REP:
		body = new(TcpData)
	case ERROR:
		body = new(ErrorData)
	}
	err = body.UnmarshalBinary(data[headerPos+1])
	if err != nil {
		return nil, err //InvalidBody
	}
	return &Msg{data[:headerPos], header, body}, nil
}

func toFrames(m *Msg) (data [][]byte, err error) {
	var header, body []byte

	header, err = m.Header.MarshalBinary()
	if err != nil {
		return
	}

	body, err = m.Body.MarshalBinary()
	if err != nil {
		return
	}

	data = make([][]byte, len(m.Envelope)+2)
	headerPos := len(m.Envelope)
	if headerPos > 0 {
		copy(data[:headerPos], m.Envelope)
	}
	data[headerPos] = header
	data[headerPos+1] = body
	return
}

func makeReqMsg(sid UID, msgType uint16, ct uint16, body []byte, flag uint16) *Msg {
	return &Msg{
		Envelope: [][]byte{[]byte("")},
		Header:   &Header{StreamId: sid, Version: VERSION_1, Flag: flag, MsgType: msgType},
		Body:     &TcpData{ContentType: ct, Payload: body},
	}
}

func makeCacheShareMsg(key []byte, digest []byte) *Msg {
	return &Msg{
		Envelope: [][]byte{[]byte("")},
		Header:   &Header{Version: VERSION_1, MsgType: CACHE_SHARE},
		Body:     &CacheShareData{Payload: []CacheItem{CacheItem{key, digest}}},
	}
}

type msgBuilder struct {
	sid          UID
	envelope     [][]byte
	defaultFlags uint16
}

func (b *msgBuilder) MakeMsg(mt uint16, ct uint16, payload []byte, flag uint16) *Msg {
	return &Msg{
		Envelope: b.envelope,
		Header:   &Header{Version: VERSION_1, StreamId: b.sid, Flag: flag | b.defaultFlags, MsgType: mt},
		Body:     &TcpData{ContentType: ct, Payload: payload},
	}
}

func (b *msgBuilder) MakeErrorMsg(err error, flag uint16) *Msg {
	flag = flag | FLAG_STREAM_END
	return &Msg{
		Envelope: b.envelope,
		Header:   &Header{Version: VERSION_1, StreamId: b.sid, Flag: flag | b.defaultFlags, MsgType: ERROR},
		Body:     &ErrorData{ContentType: CT_RAW, Payload: []byte(err.Error())},
	}
}

func NewMsgBuilder(sid UID, envelope [][]byte, flags uint16) MsgBuilder {
	return &msgBuilder{sid: sid, envelope: envelope, defaultFlags: flags}
}

func NewMsgBuilderFromMsg(tpl *Msg) MsgBuilder {
	//copy flags
	var flags uint16 = 0
	for _, cf := range []uint16{FLAG_TCP, FLAG_UDP, FLAG_HTTP} {
		if tpl.TestFlag(cf) {
			flags = flags | cf
		}
	}
	return &msgBuilder{sid: tpl.GetStreamId(), envelope: tpl.Envelope, defaultFlags: flags}
}
