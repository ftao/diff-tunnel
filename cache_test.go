package dtunnel

import (
	"reflect"
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	cacheItems := []CacheItem{
		CacheItem{[]byte("http://example.com"), []byte("123")},
		CacheItem{[]byte("http://httpbin.org"), []byte("456")},
	}
	csd := &CacheShareData{Payload: cacheItems}

	b, err := csd.MarshalBinary()
	t.Logf("result len %d %x", len(b), b)
	if err != nil {
		t.Fatalf("fail to marshalbinary %v", err)
	}

	ncsd := new(CacheShareData)
	err = ncsd.UnmarshalBinary(b)
	if err != nil {
		t.Fatalf("fail to unmarshalbinary %v", err)
	}

	if !reflect.DeepEqual(ncsd, csd) {
		t.Logf("ncsd %v", ncsd)
		t.Error("old msg & new msg should be equal")
	}
}
