package utils

import (
	"fmt"
	"net"
	"sync"
)

// PortAllocator 智能端口分配器（高位优先）
// 使用 49152-65535 动态端口范围，从高位向低位扫描
type PortAllocator struct {
	mu       sync.Mutex
	used     map[int]bool
	minPort  int
	maxPort  int
}

var (
	globalAllocator     *PortAllocator
	globalAllocatorOnce sync.Once
)

// GetPortAllocator 获取全局端口分配器单例
func GetPortAllocator() *PortAllocator {
	globalAllocatorOnce.Do(func() {
		globalAllocator = &PortAllocator{
			used:    make(map[int]bool),
			minPort: 49152, // IANA 动态端口范围下限
			maxPort: 65535, // 上限
		}
		// 扫描已占用的端口
		globalAllocator.scanUsedPorts()
	})
	return globalAllocator
}

// Allocate 分配一个可用端口（高位优先）
func (pa *PortAllocator) Allocate() (int, error) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	// 从高位向低位扫描
	for port := pa.maxPort; port >= pa.minPort; port-- {
		if pa.used[port] {
			continue
		}
		if !isPortAvailable(port) {
			continue
		}
		pa.used[port] = true
		return port, nil
	}

	return 0, fmt.Errorf("无可用端口（范围 %d-%d）", pa.minPort, pa.maxPort)
}

// Release 释放端口
func (pa *PortAllocator) Release(port int) {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	delete(pa.used, port)
}

// IsAllocated 检查端口是否已被分配
func (pa *PortAllocator) IsAllocated(port int) bool {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	return pa.used[port]
}

// ListUsed 列出所有已分配端口
func (pa *PortAllocator) ListUsed() []int {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	var list []int
	for port := range pa.used {
		list = append(list, port)
	}
	return list
}

// scanUsedPorts 扫描系统已占用的端口
func (pa *PortAllocator) scanUsedPorts() {
	// 尝试连接常用端口范围快速检测
	for port := pa.maxPort; port >= pa.minPort; port -= 100 {
		if !isPortAvailable(port) {
			pa.used[port] = true
		}
	}
}

// isPortAvailable 检查端口是否可用
func isPortAvailable(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}
