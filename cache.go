package main

import (
	"crypto/md5"
	"fmt"
	"github.com/vmihailenco/msgpack"
	"log"
	"net/http"
	"strings"
)

type Cache interface {
	Set(key []byte, value []byte) error
	Get(key []byte) ([]byte, bool)
	GetDigest(key []byte) ([]byte, bool)
	Del(key []byte) bool
	Digest(data []byte) []byte
}

type LocalCache struct {
	store map[string][]byte
}

func (c *LocalCache) Set(key []byte, value []byte) error {
	log.Printf("set local cache %x data len %d digest %x", key, len(value), c.Digest(value))
	c.store[string(key)] = value
	return nil
}

func (c *LocalCache) Get(key []byte) (value []byte, ok bool) {
	value, ok = c.store[string(key)]
	return
}

func (c *LocalCache) GetDigest(key []byte) (digest []byte, ok bool) {
	var value []byte
	value, ok = c.store[string(key)]
	if ok {
		digest = c.Digest(value)
	}
	return
}

func (c *LocalCache) Del(key []byte) bool {
	_, ok := c.store[string(key)]
	delete(c.store, string(key))
	return ok
}

func (c *LocalCache) Digest(data []byte) []byte {
	h := md5.New()
	h.Write(data)
	digest := h.Sum(nil)
	return digest
}

func makeCacheKey(req *http.Request) []byte {
	ret := []byte(req.URL.String())
	return ret
}

func makeCache() Cache {
	return &LocalCache{make(map[string][]byte)}
}

//handle cache update
type PeerCache struct {
	store map[string][]byte
}

func (rc *PeerCache) Set(key []byte, digest []byte) error {
	rc.store[fmt.Sprintf("%x", key)] = digest
	return nil
}

func (rc *PeerCache) Get(key []byte) (digest []byte, ok bool) {
	digest, ok = rc.store[fmt.Sprintf("%x", key)]
	return
}

type CacheShareData struct {
	Payload []CacheItem
}

func (d *CacheShareData) String() string {
	parts := make([]string, len(d.Payload))
	for i, item := range d.Payload {
		parts[i] = fmt.Sprintf("%x=>%x", item.CacheKey, item.Digest)
	}
	return strings.Join(parts, " ")
}

func (d *CacheShareData) MarshalBinary() (b []byte, err error) {
	b, err = msgpack.Marshal(d)
	return
}

func (d *CacheShareData) UnmarshalBinary(b []byte) (err error) {
	err = msgpack.Unmarshal(b, d)
	return
}

type CacheItem struct {
	CacheKey []byte
	Digest   []byte
}

type CacheManager struct {
	local Cache
	peers map[string]*PeerCache
}

func (cm *CacheManager) GetPeer(pid string) (peer *PeerCache, ok bool) {
	peer, ok = cm.peers[pid]
	return
}

func (cm *CacheManager) GetPeerDigest(pid string, key []byte) (digest []byte, ok bool) {
	var peer *PeerCache
	peer, ok = cm.peers[pid]
	if !ok {
		return
	}
	digest, ok = peer.Get(key)
	return
}

func (cm *CacheManager) UpdatePeer(pid string, item *CacheItem) error {
	pc, ok := cm.peers[pid]
	if !ok {
		pc = &PeerCache{make(map[string][]byte)}
		cm.peers[pid] = pc
	}
	return pc.Set(item.CacheKey, item.Digest)
}

type CacheWorker struct {
	cm      *CacheManager
	reqChan chan *Msg
}

func (w *CacheWorker) GetReqChannel() chan *Msg {
	return w.reqChan
}

func (w *CacheWorker) Run(repChan chan *Msg) error {
	for msg := range w.reqChan {
		w.updatePeer(msg)
	}
	return nil
}

func (w *CacheWorker) updatePeer(msg *Msg) error {
	id := "global"
	cd := msg.Body.(*CacheShareData)
	log.Printf("update peer cache:%v", cd)
	for _, item := range cd.Payload {
		err := w.cm.UpdatePeer(id, &item)
		if err != nil {
			return err
		}
	}
	return nil
}

func makeCacheManager() *CacheManager {
	return &CacheManager{
		local: &LocalCache{make(map[string][]byte)},
		peers: make(map[string]*PeerCache),
	}
}

func NewCacheWorker(cm *CacheManager) *CacheWorker {
	return &CacheWorker{cm, make(chan *Msg)}
}
