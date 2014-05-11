package main

import (
	"net/http"
)

type CacheItem struct {
	Version string
	Value   []byte
}

type Cache interface {
	Set(key string, value *CacheItem)
	Get(key string) (*CacheItem, bool)
	Del(key string) bool
}

type LocalCache struct {
	store map[string]*CacheItem
}

func (c *LocalCache) Set(key string, value *CacheItem) {
	c.store[key] = value
}

func (c *LocalCache) Get(key string) (value *CacheItem, ok bool) {
	value, ok = c.store[key]
	return
}

func (c *LocalCache) Del(key string) bool {
	_, ok := c.store[key]
	delete(c.store, key)
	return ok
}

func makeCacheKey(req *http.Request) string {
	return req.URL.String()
}

func makeCache() Cache {
	return &LocalCache{make(map[string]*CacheItem)}
}
