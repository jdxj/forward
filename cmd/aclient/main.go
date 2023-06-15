package main

import (
	"flag"

	"github.com/jdxj/forward"
)

var (
	bCtlAddr = flag.String("bCtlAddr", "127.0.0.1:8082", "b control address")
)

func main() {
	flag.Parse()

	a := forward.NewA(*bCtlAddr)
	_ = a.Start()

	forward.CaptureSignal()

	a.Stop()
}
