package main

import (
	"log"
)

func serverMain(bind string, pub string, secret string) {
	ts, _ := NewTunnelServerKeyPair(bind, pub, secret)
	log.Fatal(ts.Run())
}
