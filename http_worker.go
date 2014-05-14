package main

import (
	"bufio"
	"bytes"
	"log"
	"net/http"
)

type HttpWorker struct {
	reqChan chan interface{}
	repChan chan interface{}
	rt      http.RoundTripper
	cache   Cache
}

func NewHttpWorker(repChan chan interface{}) *HttpWorker {
	reqChan := make(chan interface{}, 1)
	return &HttpWorker{reqChan, repChan, &http.Transport{}, makeCache()}
}

func (hw *HttpWorker) Run() {
	for {
		req := <-hw.reqChan
		go func() {
			hw.handle(req.([][]byte))
		}()
	}
}

func (hw *HttpWorker) SendRequest(req [][]byte) {
	hw.reqChan <- req
}

func (hw *HttpWorker) handle(req [][]byte) {
	var rh RequestHeader
	_ = UnmarshalBinary(&rh, []byte(req[3]))

	reader := bufio.NewReader(bytes.NewBuffer(req[4]))
	httpReq, _ := http.ReadRequest(reader)
	httpRep, err := hw.rt.RoundTrip(httpReq)

	var header []byte
	var body []byte

	var ph ResponseHeader
	if err != nil {
		ph = ResponseHeader{Action: REP_ERROR}
		header, _ = MarshalBinary(&ph)
		body = []byte(err.Error())
		hw.repChan <- [][]byte{req[0], req[1], req[2], header, body}
	}

	//no cache, just stream
	if rh.NoCache {
		ph = ResponseHeader{Action: HTTP_DATA}
		header, _ = MarshalBinary(&ph)
		prefix := [][]byte{req[0], req[1], req[2], header}
		writer := bufio.NewWriter(&ChannelWriterCloser{hw.repChan, prefix})
		httpRep.Write(writer)
		writer.Flush()

		ph = ResponseHeader{Action: HTTP_END}
		header, _ = MarshalBinary(&ph)
		hw.repChan <- [][]byte{req[0], req[1], req[2], header, []byte("")}

	} else {
		var buff bytes.Buffer
		httpRep.Write(&buff)
		body = buff.Bytes()
		hit, newVersion, newBody := hw.updateWithCache(&rh, body)
		body = newBody
		if hit {
			ph = ResponseHeader{Action: HTTP_DIFF_REP, CacheKey: rh.CacheKey, Version: newVersion, IsPatch: hit, PatchTo: rh.Version}
		} else {
			ph = ResponseHeader{Action: HTTP_REP, CacheKey: rh.CacheKey, Version: newVersion, IsPatch: hit}
		}
		header, _ = MarshalBinary(&ph)
		hw.repChan <- [][]byte{req[0], req[1], req[2], header, body}
	}
}

//check for cache , if possible make diff, and send back diff
func (hw *HttpWorker) updateWithCache(rh *RequestHeader, body []byte) (hit bool, newVersion []byte, newBody []byte) {
	newVersion = hw.cache.GenVersion(rh.CacheKey)
	cacheItem, ok := hw.cache.Get(rh.CacheKey)
	hit = ok && bytes.Equal(cacheItem.Version, rh.Version)
	updateCache := true

	if hit {
		log.Printf("Hit Cache : %s , %s", rh.CacheKey, rh.Version)
		if bytes.Equal(cacheItem.Value, body) {
			updateCache = false
			newVersion = rh.Version
		}
		newBody = MakeDiff(cacheItem.Value, body)
	} else {
		newBody = body
	}

	if updateCache {
		log.Printf("Set Cache : %s , %s", rh.CacheKey, newVersion)
		hw.cache.Set(rh.CacheKey, &CacheItem{newVersion, body})
	}
	return
}
