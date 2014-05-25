package main

import (
	"github.com/docopt/docopt-go"
	dtunnel "github.com/ftao/diff-tunnel"
	zmq "github.com/pebbe/zmq4"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

func makeZmqStyleAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		addr = "*" + addr
	}
	if !strings.Contains(addr, "://") {
		return "tcp://" + addr
	} else {
		return addr
	}
}

func loadKeyPair(name string) (pub string, secret string, err error) {
	var data []byte
	data, err = ioutil.ReadFile(name + ".pub")
	if err != nil {
		return
	}
	pub = string(data)
	data, err = ioutil.ReadFile(name + ".key")
	if err != nil {
		return
	}
	secret = string(data)
	return
}

func serverMain(bind string, pub string, secret string) {
	ts, _ := dtunnel.NewTunnelServerKeyPair(bind, pub, secret)
	log.Fatal(ts.Run())
}

func clientMain(listen string, backend string, serverPub string, pub string, secret string) {
	tc, _ := dtunnel.NewTunnelClientKeyPair(backend, serverPub, pub, secret)
	go tc.Run()
	s := dtunnel.NewHttpProxyServer(tc)
	log.Fatal(s.ListenAndServe(listen))
}

func main() {
	usage := `diff-tunnel

Usage:
  diff-tunnel client [--http <HTTP_LISTEN>] [--backend <BACKEND>]
  diff-tunnel server [--tunnel <LISTEN>]
  diff-tunnel proxy  [--http <HTTP_LISTEN>]
  diff-tunnel genkey NAME
  diff-tunnel -h | --help
  diff-tunnel --version

Options:
  --backend=<BACKEND>        Backend Tunnel Server Endpoint [default: 127.0.0.1:8081].
  --http=<HTTP_LISTEN>       HTTP Proxy Listen Address [default: :8080].
  --tunnel=<TUNNEL_LISTEN>   Tunnel Listen Address [default: *:8081].
  -h --help                  Show this screen.
  --version                  Show version.`

	args, _ := docopt.Parse(usage, nil, true, "diff-tunnel 0.1", false)

	switch {
	case args["genkey"].(bool):
		public, secret, err := zmq.NewCurveKeypair()
		if err != nil {
			log.Printf("fail to generate key", err)
			os.Exit(1)
		}
		log.Printf("generate key pari %s.pub, %s.key", args["NAME"].(string), args["NAME"].(string))
		ioutil.WriteFile(args["NAME"].(string)+".key", []byte(secret), os.ModePerm)
		ioutil.WriteFile(args["NAME"].(string)+".pub", []byte(public), os.ModePerm)
	case args["proxy"].(bool):
		inprocAddr := "inproc://diff-tunnel"
		go serverMain(inprocAddr, "", "")
		clientMain(args["--http"].(string), inprocAddr, "", "", "")
	case args["client"].(bool):
		pub, secret, _ := loadKeyPair("client")
		serverPub, _, _ := loadKeyPair("server")
		clientMain(
			args["--http"].(string),
			makeZmqStyleAddr(args["--backend"].(string)),
			serverPub,
			pub,
			secret,
		)
	case args["server"].(bool):
		pub, secret, _ := loadKeyPair("server")
		serverMain(
			makeZmqStyleAddr(args["--tunnel"].(string)),
			pub,
			secret,
		)
	}
}
