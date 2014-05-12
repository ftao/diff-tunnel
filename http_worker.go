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
			rep := hw.handle(req.([][]byte))
			hw.repChan <- rep
		}()
	}
}

func (hw *HttpWorker) SendRequest(req [][]byte) {
	hw.reqChan <- req
}

func (hw *HttpWorker) handle(req [][]byte) [][]byte {
	var rh RequestHeader
	_ = UnmarshalBinary(&rh, []byte(req[3]))

	reader := bufio.NewReader(bytes.NewBuffer(req[4]))
	httpReq, _ := http.ReadRequest(reader)
	httpRep, err := hw.rt.RoundTrip(httpReq)

	rep := [][]byte{
		req[0],
		req[1],
		[]byte(""),
		[]byte(""),
		[]byte(""),
	}
	var header []byte
	var body []byte

	var ph ResponseHeader

	if err == nil {
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
	} else {
		ph = ResponseHeader{Action: REP_ERROR}
		body = []byte(err.Error())
	}

	log.Printf("send back header %s, body length %s", ph, len(body))
	header, _ = MarshalBinary(&ph)

	rep[3] = header
	rep[4] = body
	return rep
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
