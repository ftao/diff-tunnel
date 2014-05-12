package main

import (
	codec "github.com/ugorji/go/codec"
	//"log"
)

const (
	TCP_CONNECT     int = 1
	TCP_SEND        int = 2
	HTTP_REQ        int = 3
	TCP_CONNECT_REP int = 4
	HTTP_REP        int = 5
	HTTP_DIFF_REP   int = 6
	REP_ERROR       int = 255
)

type RequestHeader struct {
	Action   int
	CacheKey string
	Version  []byte
}

type ResponseHeader struct {
	Action   int
	CacheKey string
	IsPatch  bool
	Version  []byte
	PatchTo  []byte
}

func MarshalBinary(v interface{}) (data []byte, err error) {
	var mh codec.MsgpackHandle
	enc := codec.NewEncoderBytes(&data, &mh)
	err = enc.Encode(v)
	return
}

func UnmarshalBinary(v interface{}, data []byte) (err error) {
	var mh codec.MsgpackHandle
	dec := codec.NewDecoderBytes(data, &mh)
	err = dec.Decode(v)
	return
}
