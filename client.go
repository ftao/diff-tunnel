package main

import (
	"github.com/elazarl/goproxy"
	"log"
	"net/http"
)

type HttpProxyServer struct {
	proxy *goproxy.ProxyHttpServer
	rt    http.RoundTripper
}

var isMethodGetOrHeader = goproxy.ReqConditionFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) bool {
	return r.Method == "GET" || r.Method == "HEAD"
})

func (s *HttpProxyServer) ListenAndServe(bind string) error {
	s.proxy.Verbose = true
	s.proxy.OnRequest(isMethodGetOrHeader).DoFunc(s.handleRequest)
	return http.ListenAndServe(bind, s.proxy)
}

func (s *HttpProxyServer) handleRequest(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	resp, _ := s.rt.RoundTrip(r)
	return r, resp
}

func clientMain(listen string, backend string) {
	tc, _ := NewTunnelClient(backend)
	go tc.Run()
	proxy := goproxy.NewProxyHttpServer()

	s := &HttpProxyServer{proxy, tc}
	log.Fatal(s.ListenAndServe(listen))
}
