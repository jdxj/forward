package forward

import (
	"bytes"
	"flag"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPacket_Encode(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	p := &Packet{
		Data: map[string]string{
			"abc": "def",
		},
	}

	err := p.Encode(buf)
	if err != nil {
		t.Fatalf("%s\n", err)
	}

	p2 := &Packet{
		Data: map[string]string{
			"123": "456",
		},
	}
	err = p2.Encode(buf)
	if err != nil {
		t.Fatalf("%s\n", err)
	}

	fmt.Printf("%s\n", buf.String())

	var p3 Packet
	err = p3.Decode(buf)
	if err != nil {
		t.Fatalf("%s\n", err)
	}
	fmt.Printf("%+v\n", p3)
}

func TestFlag(t *testing.T) {
	ts := flag.NewFlagSet("test", flag.ContinueOnError)
	ts.String("abc", "def", "")

	err := ts.Parse([]string{"-abc", "hah"})
	if err != nil {
		t.Fatalf("%s\n", err)
	}

	fmt.Printf("%s\n", ts.Lookup("abc").Value)
}

func TestSplit(t *testing.T) {
	str := "test -abc def"
	parts := strings.SplitN(str, " ", 2)
	fmt.Printf("%v, %d\n", parts, len(parts))
	for _, v := range parts {
		fmt.Println(v)
	}
}

func TestReadDeadline(t *testing.T) {
	conn, err := net.Dial("tcp", "127.0.0.1:8080")
	if err != nil {
		t.Fatalf("%s\n", err)
	}
	defer conn.Close()

	// go func() {
	// 	time.Sleep(time.Second)
	// 	conn.Close()
	// }()

	buf := make([]byte, 1024)
	for {

		_, err = conn.Read(buf)
		if err != nil {
			t.Fatalf("%s\n", err)
		}
	}
}

func TestAtomicValue(t *testing.T) {
	v := atomic.Value{}
	i, ok := v.Load().(int)
	fmt.Printf("%d, %t\n", i, ok)
}

func TestCond(t *testing.T) {
	c := sync.NewCond(&sync.Mutex{})

	f := false

	go func() {
		c.L.Lock()
		f = true
		c.L.Unlock()
		c.Signal()
	}()

	c.L.Lock()
	for !f {
		fmt.Printf("not ok\n")
		c.Wait()
	}
	c.L.Unlock()
}

func TestUntil(t *testing.T) {
	s := time.Now()
	time.Sleep(time.Second)
	u := time.Since(s)
	fmt.Printf("%s\n", u)
}
