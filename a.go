package forward

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"syscall"
	"time"
)

var (
	DialTimeout = time.Second * 5
)

func NewA(bCtlAddr string) *A {
	c := &A{
		bCtlAddr: bCtlAddr,
		bCtlLock: sync.NewCond(&sync.Mutex{}),
		stop:     make(chan struct{}),
	}

	return c
}

type A struct {
	bCtlAddr string
	bCtlLock *sync.Cond
	bCtlConn net.Conn

	// key: *Tunnel, value: nil
	badTunnelMap sync.Map

	stop chan struct{}
	wg   sync.WaitGroup
}

func (c *A) connect() {
	var (
		interval = time.Second * 2
		timer    = time.NewTimer(interval)
	)
	defer timer.Stop()

	for {
		timer.Reset(interval)
		select {
		case <-c.stop:
			return
		case <-timer.C:
		}

		bCtlConn, err := net.DialTimeout("tcp", c.bCtlAddr, DialTimeout)
		if err != nil {
			if errors.Is(err, syscall.ECONNREFUSED) {
				// b没启动, 直接退出
				// todo: 直接退出的逻辑不太好
				log.Fatalln("connection refused, stop retry dial")
			}
			log.Printf("dial %s err: %s", c.bCtlAddr, err)
			continue
		}
		log.Printf("connect bCtlAddr [%s] success\n", c.bCtlAddr)
		// 使用Cond确保c.bCtlConn持有当前连接,
		// c.bCtlConn用于释放连接, 避免该goroutine阻塞在Encode()无法退出.
		c.bCtlLock.L.Lock()
		c.bCtlConn = bCtlConn
		c.bCtlLock.L.Unlock()
		c.bCtlLock.Broadcast()

		encoder := json.NewEncoder(bCtlConn)
		decoder := json.NewDecoder(bCtlConn)

		for {
			// 1. 解码
			p := &Packet{}
			err := decoder.Decode(p)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					log.Println("stop decode")
				} else if errors.Is(err, io.EOF) {
					log.Println("bCtlConn closed")
				} else if errors.Is(err, syscall.ECONNRESET) {
					log.Println("invalid bCtlConn")
				} else {
					log.Printf("decode packet err: %s", err)
				}
				break
			}
			// 2. 处理
			if p.Data == nil {
				p.Data = make(map[string]string)
			}
			p = c.handle(p)
			// 3. 编码
			err = encoder.Encode(p)
			if err != nil {
				// todo: 断言其他错误
				log.Printf("encode packet err: %s", err)
				break
			}
		}

		_ = bCtlConn.Close()
	}
}

// handle 命令处理
func (c *A) handle(req *Packet) *Packet {
	rsp := &Packet{
		Data: make(map[string]string),
	}

	switch req.Cmd {
	case "hello":
		rsp.Cmd = "world"
	case "testD":
		return c.tcpPingD(req)
	case "tunnelBAD":
		return c.addBADTunnel(req)
	default:
		rsp.Cmd = "error"
		rsp.Data["msg"] = fmt.Sprintf("[%s] not registered", req.Cmd)
	}
	return rsp
}

// tcpPingD 测试目标地址是否可用
func (c *A) tcpPingD(req *Packet) *Packet {
	var (
		start = time.Now()
		dAddr = req.Data["dAddr"]
	)
	conn, err := net.DialTimeout("tcp", dAddr, DialTimeout)
	if err != nil {
		req.Data["error"] = err.Error()
		return req
	}
	_ = conn.Close()

	req.Data["msg"] = fmt.Sprintf("tcp ping since: %s", time.Since(start))
	return req
}

// addBADTunnel 建立BAD隧道
func (c *A) addBADTunnel(req *Packet) *Packet {
	var (
		bAddr = req.Data["bAddr"]
		dAddr = req.Data["dAddr"]
	)
	baConn, err := net.DialTimeout("tcp", bAddr, DialTimeout)
	if err != nil {
		req.Data["msg"] = fmt.Sprintf("dial %s err: %s", bAddr, err)
		return req
	}
	adConn, err := net.DialTimeout("tcp", dAddr, DialTimeout)
	if err != nil {
		req.Data["msg"] = fmt.Sprintf("dial [%s] err: %s", dAddr, err)
		// 释放连接
		_ = baConn.Close()
		return req
	}

	t := NewTunnel(baConn, adConn)
	if err := t.Connect(); err != nil {
		req.Data["msg"] = fmt.Sprintf("connect bad tunnel err: %s", err)
		return req
	}
	c.badTunnelMap.Store(t, nil)
	log.Printf("add bad tunnel\nbaConn: %s\nadConn: %s\n",
		formatConnInfo(baConn),
		formatConnInfo(adConn),
	)

	req.Data["msg"] = fmt.Sprintf("add bad tunnel success")
	req.Data["code"] = "1"
	return req
}

// clearInvalidBADTunnel 清理无效隧道
func (c *A) clearInvalidBADTunnel() {
	var (
		interval = time.Second * 5
		timer    = time.NewTimer(interval)
	)
	defer timer.Stop()

	for {
		timer.Reset(interval)
		select {
		case <-c.stop:
			return
		case <-timer.C:
		}

		c.badTunnelMap.Range(func(key, value any) bool {
			badTunnel := key.(*Tunnel)
			if badTunnel.Status() == Disconnected {
				c.badTunnelMap.Delete(badTunnel)
				log.Printf("delete bad tunnel: %p\n", badTunnel)
			}
			return true
		})
	}
}

// clearBADTunnel 删除隧道
func (c *A) clearBADTunnel() {
	c.badTunnelMap.Range(func(key, value any) bool {
		badTunnel := key.(*Tunnel)
		c.badTunnelMap.Delete(badTunnel)
		_ = badTunnel.Disconnect()
		return true
	})
}

func (c *A) Start() error {
	c.wg.Add(1)
	go func() {
		c.wg.Done()

		c.connect()
	}()

	c.wg.Add(1)
	go func() {
		c.wg.Done()

		c.clearInvalidBADTunnel()
	}()

	return nil
}

func (c *A) Stop() {
	// 关闭baConn
	c.bCtlLock.L.Lock()
	for c.bCtlConn == nil {
		log.Println("wait for baConn")
		c.bCtlLock.Wait()
	}
	_ = c.bCtlConn.Close()
	c.bCtlLock.L.Unlock()

	close(c.stop)
	c.clearBADTunnel()
	c.wg.Wait()
}
