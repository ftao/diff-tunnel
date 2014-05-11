package main

import (
	"log"
)

func serverMain(bind string) {
	ts, _ := NewTunnelServer(bind)
	log.Fatal(ts.Run())
}
