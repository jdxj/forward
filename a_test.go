package forward

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestMultiEncode(t *testing.T) {
	e := json.NewEncoder(os.Stdout)
	packet := &Packet{}
	for i := 0; i < 3; i++ {
		packet.Cmd = "int"
		packet.Data = map[string]string{
			"num": strconv.Itoa(i),
		}
		err := e.Encode(packet)
		if err != nil {
			t.Fatalf("%s\n", err)
		}
	}
}

/*
 */

var (
	testStr = `
{"Cmd":"test","Data":{"dAddr":"127.0.0.1:8081"}}
{"Cmd":"tunnel","Data":{"bAddr":"127.0.0.1:8082","dAddr":"127.0.0.1:8081"}}
{"Cmd":"int","Data":{"num":"2"}}
`
)

func TestMultiDecode(t *testing.T) {
	r := strings.NewReader(testStr)
	d := json.NewDecoder(r)
	packet := &Packet{}
	for i := 0; i < 3; i++ {
		err := d.Decode(packet)
		if err != nil {
			t.Fatalf("%s\n", err)
		}
		fmt.Printf("%+v\n", packet)
	}
}

func TestValues(t *testing.T) {
	v := url.Values{}
	fmt.Printf("[%s]\n", v.Encode())
}
