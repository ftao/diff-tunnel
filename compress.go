package dtunnel

import (
	"bytes"
	"errors"
	"github.com/vmihailenco/msgpack"
	"io"
	"log"
)

var ErrorCompressFail = errors.New("CompressFail")
var ErrorDecompressFail = errors.New("DecompressFail")

type CompressorWriter interface {
	io.WriteCloser
}

type Compressor interface {
	CompressorWriter
	Bytes() []byte
	WriteTo(w io.Writer) (int64, error)
}

type CacheCompressorWriter struct {
	cache       Cache
	cacheKey    []byte
	cacheDigest []byte
	buff        *bytes.Buffer
	writer      io.Writer
	update      bool
}

func (c *CacheCompressorWriter) Write(b []byte) (n int, err error) {
	n, err = c.buff.Write(b)
	return
}

func (c *CacheCompressorWriter) Flush() error {
	//doing nothhing
	return nil
}

func (c *CacheCompressorWriter) Close() error {
	_, compressed := c.compress(c.buff.Bytes())
	//TODO: make sure all is written
	c.writer.Write(compressed)

	if c.update {
		c.cache.Set(c.cacheKey, c.buff.Bytes())
	}
	return nil
}

func (c *CacheCompressorWriter) compress(body []byte) (hit bool, data []byte) {
	cacheDigest, ok := c.cache.GetDigest(c.cacheKey)
	hit = ok && bytes.Equal(cacheDigest, c.cacheDigest)
	log.Printf("compress hit %t key %x remote %x local %x", hit, c.cacheKey, c.cacheDigest, cacheDigest)
	if hit {
		cacheBody, _ := c.cache.Get(c.cacheKey)
		data = MakeDiff(cacheBody, body)
		diff := &DiffContent{c.cacheKey, cacheDigest, data}
		data, _ = msgpack.Marshal(diff)
	} else {
		diff := &DiffContent{[]byte(""), []byte(""), body}
		data, _ = msgpack.Marshal(diff)
	}
	return
}

func NewCacheCompressorWriter(writer io.Writer, cache Cache, cacheKey []byte, cacheDigest []byte, update bool) io.WriteCloser {
	return &CacheCompressorWriter{
		cache:       cache,
		cacheKey:    cacheKey,
		cacheDigest: cacheDigest,
		buff:        new(bytes.Buffer),
		writer:      writer,
		update:      update,
	}
}

type CacheCompressor struct {
	*CacheCompressorWriter
	writeTo *bytes.Buffer
}

func (c *CacheCompressor) Bytes() []byte {
	return c.writeTo.Bytes()
}

func (c *CacheCompressor) WriteTo(w io.Writer) (n int64, err error) {
	return c.writeTo.WriteTo(w)
}

func NewCacheCompressor(cache Cache, cacheKey []byte, cacheDigest []byte, update bool) Compressor {
	buff := new(bytes.Buffer)
	cwriter := &CacheCompressorWriter{
		cache:       cache,
		cacheKey:    cacheKey,
		cacheDigest: cacheDigest,
		buff:        new(bytes.Buffer),
		writer:      buff,
		update:      update,
	}
	return &CacheCompressor{cwriter, buff}
}

//TODO: should pass in []byte
func decompress(cache Cache, dc *DiffContent) (data []byte, err error) {
	cacheDigest, ok := cache.GetDigest(dc.CacheKey)
	hit := ok && bytes.Equal(cacheDigest, dc.PatchTo)
	if hit {
		cacheBody, _ := cache.Get(dc.CacheKey)
		data = Patch(cacheBody, dc.Diff)
		cache.Set(dc.CacheKey, data)
	} else {
		log.Printf("decompress fail , cache digest %s not match", dc.PatchTo)
		err = ErrorDecompressFail
	}
	return
}
