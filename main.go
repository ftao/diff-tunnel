package main

import (
	"fmt"
	"github.com/docopt/docopt-go"
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

func main() {
	usage := `diff-tunnel

Usage:
  diff-tunnel client [--http <HTTP_LISTEN>] [--backend <BACKEND>]
  diff-tunnel server [--tunnel <LISTEN>]
  diff-tunnel proxy  [--http <HTTP_LISTEN>]
  diff-tunnel -h | --help
  diff-tunnel --version

Options:
  --backend=<BACKEND>        Backend Tunnel Server Endpoint [default: 127.0.0.1:8081].
  --http=<HTTP_LISTEN>       HTTP Proxy Listen Address [default: :8080].
  --tunnel=<TUNNEL_LISTEN>   Tunnel Listen Address [default: *:8081].
  -h --help                  Show this screen.
  --version                  Show version.`

	args, _ := docopt.Parse(usage, nil, true, "diff-tunnel 0.1", false)
	fmt.Println(args)

	if args["proxy"].(bool) {
		inprocAddr := "inproc://diff-tunnel"
		go serverMain(inprocAddr)
		clientMain(args["--http"].(string), inprocAddr)
	}
	if args["client"].(bool) {
		clientMain(args["--http"].(string),
			makeZmqStyleAddr(args["--backend"].(string)))
	}
	if args["server"].(bool) {
		serverMain(makeZmqStyleAddr(args["--tunnel"].(string)))
	}

}
