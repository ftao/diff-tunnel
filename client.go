package main

import (
	"bufio"
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

type TcpTransport interface {
	ConnectTcp(address string) (net.Conn, error)
}

type HttpTransport interface {
	RoundTrip(r *http.Request) (io.ReadCloser, error)
}

type HttpProxyServer struct {
	ht HttpTransport
	tt TcpTransport
}

func (s *HttpProxyServer) ListenAndServe(bind string) error {
	return http.ListenAndServe(bind, s)
}

func (s *HttpProxyServer) hijack(w http.ResponseWriter, r *http.Request) (net.Conn, string) {
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

	return proxyClient, host
}

func (s *HttpProxyServer) handleHttp(w http.ResponseWriter, r *http.Request) {
	reader, err := s.ht.RoundTrip(r)
	if err != nil {
		log.Printf("error got response %v", err)
		return
	}
	resp, err := http.ReadResponse(bufio.NewReader(reader), r)
	if err != nil {
		log.Printf("error got response %v", err)
		return
	}
	defer reader.Close()

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("error copy to client %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		log.Printf("Can't close response body %v", err)
	}
}

func (s *HttpProxyServer) handleHttps(w http.ResponseWriter, r *http.Request) {
	proxyClient, host := s.hijack(w, r)
	remote, err := s.tt.ConnectTcp(host)
	if err == nil {
		proxyClient.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))
		piping(proxyClient, remote)
	} else {
		proxyClient.Write([]byte("HTTP/1.0 502 Bad Gateway\r\n\r\n"))
	}
}

func (s *HttpProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "CONNECT" {
		s.handleHttps(w, r)
	} else {
		s.handleHttp(w, r)
	}
}

func clientMain(listen string, backend string, serverPub string, pub string, secret string) {
	tc, _ := NewTunnelClientKeyPair(backend, serverPub, pub, secret)
	go tc.Run()
	s := &HttpProxyServer{tc, tc}
	log.Fatal(s.ListenAndServe(listen))
}
