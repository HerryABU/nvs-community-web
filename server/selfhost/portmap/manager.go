// Package portmap 实现四层 TCP 端口映射的运行时：监听端口 → 双向转发到目标地址。
// 与路径级 HTTP 代理独立；只由管理员配置，错误通过回调回写数据库。
package portmap

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type Endpoint struct {
	ID         uint
	ListenPort int
	Target     string
}

type runtime struct {
	ep     Endpoint
	ln     net.Listener
	closed chan struct{}
	once   sync.Once
}

func (r *runtime) stop() {
	r.once.Do(func() {
		close(r.closed)
		_ = r.ln.Close()
	})
}

// Manager 保存当前生效的监听实例；Reload 时只变更新增/修改/删除的部分。
type Manager struct {
	mu     sync.Mutex
	active map[uint]*runtime
}

func New() *Manager { return &Manager{active: map[uint]*runtime{}} }

func (m *Manager) Running(id uint) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.active[id]
	return ok
}

func (m *Manager) Reload(endpoints []Endpoint, onError func(uint, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	desired := make(map[uint]Endpoint, len(endpoints))
	for _, ep := range endpoints {
		desired[ep.ID] = ep
	}
	// 先停掉已删除或已变更的监听。
	for id, r := range m.active {
		ep, ok := desired[id]
		if !ok || ep.ListenPort != r.ep.ListenPort || ep.Target != r.ep.Target {
			r.stop()
			delete(m.active, id)
		}
	}
	// 再启动新的监听。
	for id, ep := range desired {
		if _, exists := m.active[id]; exists {
			continue
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", ep.ListenPort))
		if err != nil {
			log.Printf("[portmap] 监听 %d 失败: %v", ep.ListenPort, err)
			if onError != nil {
				onError(id, err)
			}
			continue
		}
		r := &runtime{ep: ep, ln: ln, closed: make(chan struct{})}
		m.active[id] = r
		if onError != nil {
			onError(id, nil)
		}
		go r.accept()
	}
}

func (r *runtime) accept() {
	for {
		client, err := r.ln.Accept()
		if err != nil {
			select {
			case <-r.closed:
				return
			default:
				continue
			}
		}
		go r.pipe(client)
	}
}

func (r *runtime) pipe(client net.Conn) {
	backend, err := net.DialTimeout("tcp", r.ep.Target, 5*time.Second)
	if err != nil {
		_ = client.Close()
		return
	}
	var wg sync.WaitGroup
	copyOne := func(dst, src net.Conn) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		if c, ok := dst.(*net.TCPConn); ok {
			_ = c.CloseWrite()
		} else {
			_ = dst.Close()
		}
	}
	wg.Add(2)
	go copyOne(backend, client)
	go copyOne(client, backend)
	wg.Wait()
	_ = client.Close()
	_ = backend.Close()
}
