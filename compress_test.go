package dtunnel

import (
	"bytes"
	"github.com/vmihailenco/msgpack"
	"log"
	"testing"
)

func TestCompressHit(t *testing.T) {
	cache := makeCache()
	key := []byte("http://www.example.com")
	value := []byte("hello world \n")
	digest := cache.Digest(value)
	cache.Set(key, value)

	comp := NewCacheCompressor(cache, key, digest, false)
	comp.Write([]byte("hello world \n goodbye"))
	comp.Close()

	result := comp.Bytes()

	diff := new(DiffContent)
	err := msgpack.Unmarshal(result, diff)
	if err != nil {
		t.Errorf("comp result is not valid, %s", err)
	}

	if !bytes.Equal(diff.PatchTo, digest) {
		t.Errorf("comp result patch is not right, expected %x , got %x", digest, diff.PatchTo)
	}
	if !bytes.Equal(diff.CacheKey, []byte(key)) {
		t.Errorf("comp result cachekey is not right, expected %x , got %x", key, diff.CacheKey)
	}

	log.Printf("cache diff %s", diff.Diff)
}
