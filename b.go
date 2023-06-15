package forward

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
)

func NewB(bIP, cCtlPort, aCtlPort string) *B {
	b := &B{
		stop:     make(chan struct{}),
		bIP:      bIP,
		cCtlPort: cCtlPort,
		aCtlPort: aCtlPort,
	}
	return b
}

type B struct {
	wg   sync.WaitGroup
	stop chan struct{}

	// b的IP
	bIP string

	cCtlPort string
	cCtlLis  net.Listener
	// 只持有一个连接
	cCtlConn atomic.Value

	aCtlPort string
	aCtlLis  net.Listener
	aCtlConn atomic.Value

	// 隧道Listener
	// key:dAddr, value:CAListener
	caLisMap sync.Map
}

func (b *B) listenCCtl() (err error) {
	b.cCtlLis, err = net.Listen("tcp", ":"+b.cCtlPort)
	if err != nil {
		return
	}

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()

		for {
			conn, err := b.cCtlLis.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					log.Println("stop cbLis accept")
					return
				}
				log.Printf("cbLis accept err: %s\n", err)
				continue
			}
			// 只持有一个连接
			b.cCtlConn.Store(conn)
			// c没退出, 处理不结束
			b.handleCCtl(conn)
			_ = conn.Close()
		}
	}()

	return nil
}

func (b *B) handleCCtl(conn net.Conn) {
	parser := NewCliParser(conn, conn)
	aCtlConn, ok := b.aCtlConn.Load().(net.Conn)
	if !ok {
		p := &Packet{
			Data: map[string]string{
				"msg": fmt.Sprintf("a没连上\n"),
			},
		}
		err := parser.Encode(p)
		if err != nil {
			log.Printf("encode cli err: %s\n", err)
			return
		}
	}
	aCtlEncoder := json.NewEncoder(aCtlConn)
	aCtlDecoder := json.NewDecoder(aCtlConn)

	for {
		req, err := parser.Decode()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Println("c ctl closed")
				return
			} else if errors.Is(err, net.ErrClosed) {
				log.Println("stop decode cli")
				return
			}
			// todo: 其他类型err断言
			log.Printf("decode cli err: %s", err)
			return
		}

		var rsp *Packet
		switch req.Cmd {
		case "":
			continue
		case "testD":
			rsp, err = testD(aCtlEncoder, aCtlDecoder, req)
		case "tunnelBAD":
			rsp, err = b.tunnelBAD(aCtlEncoder, aCtlDecoder, req)
		default:
			rsp = &Packet{Data: map[string]string{"msg": fmt.Sprintf("%s not implement", req.Cmd)}}
		}

		err = parser.Encode(rsp)
		if err != nil {
			// todo: 断言err
			log.Printf("encode cli err: %s\n", err)
			return
		}
	}
}

func testD(encoder *json.Encoder, decoder *json.Decoder, req *Packet) (*Packet, error) {
	err := encoder.Encode(req)
	if err != nil {
		return nil, err
	}

	rsp := &Packet{}
	return rsp, decoder.Decode(rsp)
}

func (b *B) tunnelBAD(encoder *json.Encoder, decoder *json.Decoder, req *Packet) (*Packet, error) {
	dAddr := req.Data["dAddr"]
	// 1. 创建listener
	caLis := NewCAListener(dAddr)
	v, loaded := b.caLisMap.LoadOrStore(dAddr, caLis)
	if !loaded {
		// 使用新创建的
		err := caLis.Start()
		if err != nil {
			b.caLisMap.Delete(dAddr)
			caLis.Stop()
			return nil, err
		}
		v = caLis
	}
	caLis = v.(*CAListener)

	// 2. 创建BAD隧道
	req.Data["bAddr"] = fmt.Sprintf("%s:%d", b.bIP, caLis.ARandPort())
	err := encoder.Encode(req)
	if err != nil {
		return nil, err
	}

	rsp := &Packet{}
	err = decoder.Decode(rsp)
	if err != nil {
		return nil, err
	}
	if rsp.Data["code"] != "1" {
		return rsp, nil
	}

	// 3. 返回cRandPort
	req.Data["randAddrForC"] = fmt.Sprintf("%s:%d", b.bIP, caLis.CRandPort())
	return req, nil
}

func (b *B) listenACtl() (err error) {
	b.aCtlLis, err = net.Listen("tcp", ":"+b.aCtlPort)
	if err != nil {
		return
	}

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()

		for {
			conn, err := b.aCtlLis.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					log.Println("stop baLis accept")
					return
				}
				log.Printf("baLis accept err: %s\n", err)
				continue
			}

			b.aCtlConn.Store(conn)
		}
	}()

	return nil
}

func (b *B) clearCBAListener() {
	b.caLisMap.Range(func(key, value any) bool {
		b.caLisMap.Delete(key)
		caLis := value.(*CAListener)
		caLis.Stop()
		return true
	})
}

func (b *B) Start() error {
	err := b.listenACtl()
	if err != nil {
		return err
	}
	return b.listenCCtl()
}

func (b *B) Stop() {
	_ = b.cCtlLis.Close()
	conn, ok := b.cCtlConn.Load().(net.Conn)
	if ok {
		_ = conn.Close()
	}

	_ = b.aCtlLis.Close()
	conn, ok = b.aCtlConn.Load().(net.Conn)
	if ok {
		_ = conn.Close()
	}

	close(b.stop)
	b.clearCBAListener()
	b.wg.Wait()
}
