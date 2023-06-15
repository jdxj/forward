package main

import (
	"flag"
	"log"

	"github.com/jdxj/forward"
)

var (
	bIP      = flag.String("bIP", "127.0.0.1", "b's ip")
	cCtlPort = flag.String("cCtlPort", "8081", "ctl port for c")
	aCtlPort = flag.String("aCtlPort", "8082", "ctl port for a")
)

func main() {
	flag.Parse()

	b := forward.NewB(*bIP, *cCtlPort, *aCtlPort)
	err := b.Start()
	if err != nil {
		log.Fatalln(err)
	}

	forward.CaptureSignal()

	b.Stop()
}
