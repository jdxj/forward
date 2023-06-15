package forward

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
)

func NewTunnel(down, up net.Conn) *Tunnel {
	t := &Tunnel{
		downstream: down,
		upstream:   up,
	}
	t.status.Store(Created)

	return t
}

const (
	// 刚创建
	Created = iota
	// 已连接
	Connected
	// 已断开
	Disconnected
)

type Tunnel struct {
	downstream net.Conn
	upstream   net.Conn

	status atomic.Value
	wg     sync.WaitGroup
}

func (t *Tunnel) Connect() error {
	if !t.status.CompareAndSwap(Created, Connected) {
		return fmt.Errorf("can not connect")
	}

	// 上行
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		copyStream(t.upstream, t.downstream)
	}()

	// 下行
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		copyStream(t.downstream, t.upstream)
	}()

	// 未经管理的goroutine
	go func() {
		t.wg.Wait()
		// 上下行断开了
		if t.status.CompareAndSwap(Connected, Disconnected) {
			_ = t.upstream.Close()
			_ = t.downstream.Close()
		}
	}()
	return nil
}

// todo: 目前是强制关闭, 可以看看其他关闭方法
func copyStream(dst, src net.Conn) {
	_, err := io.Copy(dst, src)
	var msg string
	if err == nil {
		msg = "stop copy steam"
		_ = dst.Close()
	} else if errors.Is(err, io.EOF) {
		msg = "copy stream eof"
	} else if errors.Is(err, net.ErrClosed) {
		msg = "copy closed stream"
	} else if errors.Is(err, syscall.EPIPE) {
		msg = "copy broken stream"
	} else if errors.Is(err, syscall.ECONNRESET) {
		msg = "copy reset stream"
	} else {
		msg = fmt.Sprintf("copy stream err: %s", err)
	}

	log.Printf("%s\ndst: %s\nsrc: %s\n",
		msg,
		formatConnInfo(dst),
		formatConnInfo(src),
	)
}

func formatConnInfo(conn net.Conn) string {
	return fmt.Sprintf("\n    LocalAddr:%s, RemoteAddr:%s", conn.LocalAddr(), conn.RemoteAddr())
}

// Disconnect 主动断开
func (t *Tunnel) Disconnect() error {
	if !t.status.CompareAndSwap(Connected, Disconnected) {
		return fmt.Errorf("can not disconnect")
	}

	_ = t.upstream.Close()
	_ = t.downstream.Close()
	t.wg.Wait()
	return nil
}

func (t *Tunnel) Status() int {
	return t.status.Load().(int)
}
