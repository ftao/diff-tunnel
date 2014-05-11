package main

const (
	REQ_HTTP_GET  int = 1
	REQ_CACHE_GET int = 2

	RESP_DIFF int = 1
	RESP_HTTP int = 2
)

type RequestHeader struct {
	Action   int
	CacheKey string
	Version  string
}

type ResponseHeader struct {
	Action   int
	CacheKey string
	IsPatch  bool
	Version  string
	PatchTo  string
}
