package main

import (
	"flag"
	"log"

	"github.com/jdxj/forward"
)

var (
	mode = flag.String("mode", "b", "b or a")
)

// b需要的参数
var (
	bIP      = flag.String("bIP", "127.0.0.1", "b's ip")
	cCtlPort = flag.String("cCtlPort", "8081", "ctl port for c")
	aCtlPort = flag.String("aCtlPort", "8082", "ctl port for a")
)

// a需要的参数
var (
	bCtlAddr = flag.String("bCtlAddr", "127.0.0.1:8082", "b control address")
)

func main() {
	flag.Parse()

	type star interface {
		Start() error
		Stop()
	}

	var s star
	switch *mode {
	case "b":
		s = forward.NewB(*bIP, *cCtlPort, *aCtlPort)
	case "a":
		s = forward.NewA(*bCtlAddr)
	default:
		log.Fatalf("invalid mode %s\n", *mode)
	}

	if err := s.Start(); err != nil {
		log.Fatalln(err)
	}

	forward.CaptureSignal()
	s.Stop()
}
