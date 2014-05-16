package main

import (
	"bytes"
	"testing"
	"time"
)

func TestCacheWorker(t *testing.T) {
	repChan := make(chan *Msg)

	id := "global"
	cm := makeCacheManager()
	worker := NewCacheWorker(cm)

	cacheItems := []CacheItem{
		CacheItem{[]byte("http://example.com"), []byte("123")},
		CacheItem{[]byte("http://httpbin.org"), []byte("456")},
	}

	msg := &Msg{
		Header: &Header{Version: VERSION_1, MsgType: CACHE_SHARE},
		Body:   &CacheShareData{Payload: cacheItems},
	}

	go worker.Run(repChan)

	worker.GetReqChannel() <- msg
	time.Sleep(time.Second)

	peer, ok := cm.GetPeer(id)
	if !ok {
		t.Fatalf("peer %s should exists", id)
	}
	for _, item := range cacheItems {
		v, ok := peer.Get(item.CacheKey)
		if !ok || !bytes.Equal(v, item.Digest) {
			t.Errorf("cache digest not match ok %b , got %x  expected %x", ok, v, item.Digest)
		}
	}
}
