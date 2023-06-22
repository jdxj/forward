package forward

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"
)

func NewCAListener(dAddr string) *CAListener {
	t := &CAListener{
		dAddr:    dAddr,
		cbConnCh: make(chan net.Conn, 10),
		baConnCh: make(chan net.Conn, 10),
		stop:     make(chan struct{}),
	}

	return t
}

type CAListener struct {
	// 仅用作标识
	dAddr string

	cRandPort int
	cLis      net.Listener
	cbConnCh  chan net.Conn

	aRandPort int
	aLis      net.Listener
	baConnCh  chan net.Conn

	cbaTunnelMap sync.Map

	stop chan struct{}
	wg   sync.WaitGroup
}

func (t *CAListener) CRandPort() int {
	return t.cRandPort
}
func (t *CAListener) ARandPort() int {
	return t.aRandPort
}

func (t *CAListener) listenA() (err error) {
	// :0 表示自动分配随机端口
	t.aLis, err = net.Listen("tcp", ":0")
	if err != nil {
		return
	}
	t.aRandPort = t.aLis.Addr().(*net.TCPAddr).Port

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		acceptConn(t.aLis, t.baConnCh)
	}()
	return
}

func (t *CAListener) listenC() (err error) {
	t.cLis, err = net.Listen("tcp", ":0")
	if err != nil {
		return
	}
	t.cRandPort = t.cLis.Addr().(*net.TCPAddr).Port

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		acceptConn(t.cLis, t.cbConnCh)
	}()
	return
}

// 将accept的conn发送到chan中
func acceptConn(l net.Listener, connCh chan net.Conn) {
	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				log.Printf("stop tunnel listen %s", l.Addr())
				return
			} else {
				log.Printf("%s accept err: %s", l.Addr(), err)
				continue
			}
		}

		connCh <- conn
	}
}

func (t *CAListener) createCBATunnel() {
	var (
		cbConn, baConn net.Conn
		ok             bool
	)

	for {
		if baConn, ok = <-t.baConnCh; !ok {
			log.Printf("closed baConnCh")
			break
		}
		log.Printf("发现baConn, 等待cbConn\nbaConn: %s\n", formatConnInfo(baConn))

		if cbConn, ok = <-t.cbConnCh; !ok {
			log.Printf("closed cbConnCh")
			break
		}
		log.Printf("发现cbConn\ncbConn: %s\n", formatConnInfo(cbConn))

		cbaTunnel := NewTunnel(cbConn, baConn)
		if err := cbaTunnel.Connect(); err != nil {
			log.Printf("add cba tunnel err: %s", err)
			_ = baConn.Close()
			_ = cbConn.Close()
			continue
		}

		t.cbaTunnelMap.Store(cbaTunnel, nil)
		log.Printf("通向%s的新隧道\ncbConn: %s\nbaConn: %s\n",
			t.dAddr,
			formatConnInfo(cbConn),
			formatConnInfo(baConn),
		)
	}

	// 关闭chan中的剩余连接
	for ; baConn != nil; baConn = <-t.baConnCh {
		_ = baConn.Close()
	}
	for ; cbConn != nil; cbConn = <-t.cbConnCh {
		_ = cbConn.Close()
	}
}

func (t *CAListener) Start() error {
	err := t.listenA()
	if err != nil {
		return err
	}

	err = t.listenC()
	if err != nil {
		return err
	}

	t.wg.Add(1)
	go func() {
		t.wg.Done()

		t.createCBATunnel()
	}()

	t.wg.Add(1)
	go func() {
		t.wg.Done()

		t.clearInvalidCBATunnel()
	}()
	return nil
}

func (t *CAListener) clearInvalidCBATunnel() {
	interval := time.Second * 5
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		timer.Reset(interval)
		select {
		case <-t.stop:
			return
		case <-timer.C:
		}

		t.cbaTunnelMap.Range(func(key, value any) bool {
			cbaTunnel := key.(*Tunnel)
			if cbaTunnel.Status() == Disconnected {
				t.cbaTunnelMap.Delete(cbaTunnel)
				log.Printf("delete cba tunnel: %p\n", cbaTunnel)
			}
			return true
		})
	}
}

func (t *CAListener) clearCBATunnel() {
	t.cbaTunnelMap.Range(func(key, value any) bool {
		cbaTunnel := key.(*Tunnel)
		t.cbaTunnelMap.Delete(cbaTunnel)
		_ = cbaTunnel.Disconnect()
		return true
	})
}

func (t *CAListener) Stop() {
	if t.cLis != nil {
		_ = t.cLis.Close()
	}
	if t.aLis != nil {
		_ = t.aLis.Close()
	}

	close(t.cbConnCh)
	close(t.baConnCh)
	close(t.stop)

	t.clearCBATunnel()
	t.wg.Wait()
}

func (t *CAListener) Active() bool {
	var active bool
	t.cbaTunnelMap.Range(func(key, value any) bool {
		active = true
		// 发现有活跃的隧道直接返回
		return false
	})
	return active
}
