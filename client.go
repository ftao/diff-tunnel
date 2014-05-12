package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
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

func copyAndClose(w net.Conn, r io.Reader) {
	connOk := true
	if _, err := io.Copy(w, r); err != nil {
		connOk = false
		log.Printf("Error copying to client %s", err)
	}
	if err := w.Close(); err != nil && connOk {
		log.Printf("Error closing %s", err)
	}
}

type HttpProxyServer struct {
	tc *TunnelClient
}

func (s *HttpProxyServer) ListenAndServe(bind string) error {
	return http.ListenAndServe(bind, s)
}

func (s *HttpProxyServer) handleRequest(r *http.Request) (*http.Request, *http.Response) {
	resp, _ := s.tc.RoundTrip(r)
	return r, resp
}

func (s *HttpProxyServer) handleHttps(w http.ResponseWriter, r *http.Request) {
	log.Print("handleHttps")
	hij, ok := w.(http.Hijacker)
	if !ok {
		panic("httpserver does not support hijacking")
	}

	proxyClient, _, e := hij.Hijack()
	if e != nil {
		panic("Cannot hijack connection " + e.Error())
	}

	host := r.URL.Host
	if !regexp.MustCompile(`:\d+$`).MatchString(host) {
		host += ":80"
	}

	remote, err := s.tc.Connect(host)

	log.Print("connect made", err)

	if err == nil {
		proxyClient.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))
		go copyAndClose(proxyClient, remote)
		go copyAndClose(remote, proxyClient)
	} else {
		proxyClient.Write([]byte("HTTP/1.0 500 Internl Server Error\r\n\r\n"))
		proxyClient.Write([]byte(err.Error()))
	}
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
