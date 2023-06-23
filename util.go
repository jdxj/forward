package forward

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func CaptureSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	<-ch

	log.Println("receive stop")
}
