package main

import (
	"io"
	"log"
	"net/http"
)

func copyHeaders(dst, src http.Header) {
	for k, _ := range dst {
		dst.Del(k)
	}
	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

type HttpProxyServer struct {
	rt http.RoundTripper
}

func (s *HttpProxyServer) ListenAndServe(bind string) error {
	return http.ListenAndServe(bind, s)
}

func (s *HttpProxyServer) handleRequest(r *http.Request) (*http.Request, *http.Response) {
	resp, _ := s.rt.RoundTrip(r)
	return r, resp
}

func (s *HttpProxyServer) handleHttps(w http.ResponseWriter, r *http.Request) {
	//hijack https request
}

func (s *HttpProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	//r.Header["X-Forwarded-For"] = w.RemoteAddr()
	if r.Method == "CONNECT" {
		s.handleHttps(w, r)
	} else {
		_, resp := s.handleRequest(r)
		copyHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		nr, err := io.Copy(w, resp.Body)
		if err := resp.Body.Close(); err != nil {
			log.Printf("Can't close response body %v", err)
		}
		log.Printf("Copied %v bytes to client error=%v", nr, err)
	}
}

func clientMain(listen string, backend string) {
	tc, _ := NewTunnelClient(backend)
	go tc.Run()
	s := &HttpProxyServer{tc}
	log.Fatal(s.ListenAndServe(listen))
}
