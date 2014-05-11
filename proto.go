package main

import (
	codec "github.com/ugorji/go/codec"
	//"log"
)

const (
	REQ_HTTP_GET  int = 1
	REQ_CACHE_GET int = 2

	RESP_DIFF  int = 1
	RESP_HTTP  int = 2
	RESP_ERROR int = 3
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
