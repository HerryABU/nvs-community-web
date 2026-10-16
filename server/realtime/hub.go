// Package realtime 评论区实时推送（F19 狂暴模式）：
// 基于内存 Pub/Sub 的 SSE 广播 Hub，把小说/博客下的新评论实时下发给订阅者，
// 营造类直播弹幕的「狂暴」体验。
//
// 带宽可控设计（四道闸，均为管理后台可配或固定保守值）：
//  1. 批量 flush：每条评论不立即下发，先进入各客户端的 pending 队列，
//     由 Hub 单一线程按 BatchIntervalSec 周期（或 pending 达 BatchSize）合并成
//     一个 EventBatch 一次性投递——弹幕高峰下不再一条评论一个 TCP 包；
//     用「每客户端 pending + 单线程 Ticker」而非每客户端一个 Ticker：
//     2000 客户端 = 2000 个定时器 goroutine 会是纯浪费，单线程遍历零额外成本。
//  2. 心跳 15s 一个注释行 ": ping\n\n"（仅几十字节），保活 + 探测半死连接；
//  3. 单客户端 channel 缓冲 64（Event 粒度）：投递采用 select+default 非阻塞写，
//     写不进去即判定慢客户端、立即断开，防止个别慢速读端堆积内存拖垮全局；
//  4. 全局连接数上限 MaxConns（默认 2000）：超限拒绝新连接（上层返回 503），
//     前端指数退避重试，防止连接数/文件描述符膨胀。
package realtime

import (
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// EventNameComment 单条评论事件名（SSE 帧：event: comment）
	EventNameComment = "comment"
	// EventNameBatch 批量事件信封名。SSE 处理器收到后拆成多条 comment 帧、
	// 但只做一次 Write+Flush：既省系统调用/带宽，又保持前端「一条评论一个 message」的简单语义
	EventNameBatch = "batch"

	// DefaultMaxConns 全局连接数上限默认值（2000：单人站点足够，又兜住内存/FD）
	DefaultMaxConns = 2000
	// DefaultBatchInterval 批量 flush 周期默认值（3s：弹幕观感与带宽的平衡点）
	DefaultBatchInterval = 3 * time.Second
	// DefaultBatchSize pending 累积达该条数立即 flush（不等周期），默认 20 条
	DefaultBatchSize = 20
	// ClientBufferSize 单客户端发送 channel 缓冲（Event 粒度）。
	// 64 个批量信封 ≈ 64*BatchSize 条评论的在途容量，足够覆盖弱网抖动
	ClientBufferSize = 64
	// HeartbeatInterval SSE 心跳周期（低成本保活，仅一条注释行）
	HeartbeatInterval = 15 * time.Second
)

// ErrTooManyConns 全局连接数已达上限，拒绝新订阅
var ErrTooManyConns = errors.New("实时连接数已达上限")

// Event 一条待广播事件。Name 为 SSE 事件名，Data 为任意可 JSON 序列化负载
type Event struct {
	Name string `json:"name"`
	Data any    `json:"data"`
}

// EventBatch 批量事件信封：一次 flush 携带的多条事件
type EventBatch struct {
	Events []Event `json:"events"`
}

// Client 单个订阅连接。channel 为服务端→处理器的事件通道；
// done 关闭表示该连接已失效（客户端主动退订或被服务端判定慢客户端断开），
// SSE 处理循环据此退出。
type Client struct {
	Key     string
	channel chan Event
	done    chan struct{}
	once    sync.Once

	mu      sync.Mutex
	pending []Event // 待批量 flush 的累积事件（仅 Hub 内部 goroutine 访问，仍有锁保护并发 Publish）
}

// Events 事件通道（只读）
func (c *Client) Events() <-chan Event { return c.channel }

// Done 连接失效信号（退订/被断开时关闭）
func (c *Client) Done() <-chan struct{} { return c.done }

// send 非阻塞投递。返回 false 表示 channel 缓冲已满（慢客户端）
func (c *Client) send(ev Event) bool {
	select {
	case c.channel <- ev:
		return true
	default:
		return false
	}
}

// appendPending 追加一条待发事件，返回当前 pending 长度（调用方判断是否达批量阈值）
func (c *Client) appendPending(ev Event) int {
	c.mu.Lock()
	c.pending = append(c.pending, ev)
	n := len(c.pending)
	c.mu.Unlock()
	return n
}

// takePending 取出全部待发事件（无剩余返回 nil）
func (c *Client) takePending() []Event {
	c.mu.Lock()
	evs := c.pending
	c.pending = nil
	c.mu.Unlock()
	return evs
}

// closeDone 关闭失效信号（幂等）
func (c *Client) closeDone() { c.once.Do(func() { close(c.done) }) }

// Hub 频道→客户端集合 的广播中枢。
// 并发模型：订阅表由 RWMutex 保护；pending 追加/取空由每客户端小锁保护；
// 投递一律非阻塞（select+default），任何情况下 Hub 自身都不会被慢客户端阻塞。
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]struct{}

	// 限额（原子读写，管理后台保存配置即时生效，无需重连）
	maxConns      atomic.Int64
	batchInterval atomic.Int64 // time.Duration 纳秒
	batchSize     atomic.Int64
	enabled       atomic.Bool // 狂暴模式开关（LiveStream 连接建立时按配置同步）

	stop chan struct{}
	once sync.Once
}

// NewHub 创建 Hub 并启动批量 flush 线程（默认限额）
func NewHub() *Hub {
	h := &Hub{
		clients: make(map[string]map[*Client]struct{}),
		stop:    make(chan struct{}),
	}
	h.maxConns.Store(DefaultMaxConns)
	h.batchInterval.Store(int64(DefaultBatchInterval))
	h.batchSize.Store(DefaultBatchSize)
	go h.flushLoop()
	return h
}

// Close 停止 flush 线程（测试用；生产进程常驻不关闭）
func (h *Hub) Close() { h.once.Do(func() { close(h.stop) }) }

// SetLimits 运行时更新限额（管理后台保存 / SSE 连接建立时同步，立即生效）
func (h *Hub) SetLimits(maxConns int, batchInterval time.Duration, batchSize int) {
	if maxConns > 0 {
		h.maxConns.Store(int64(maxConns))
	}
	if batchInterval > 0 {
		h.batchInterval.Store(int64(batchInterval))
	}
	if batchSize > 0 {
		h.batchSize.Store(int64(batchSize))
	}
}

// SetEnabled 设置狂暴模式开关（推送层生效：关闭后不再向任何连接广播）
func (h *Hub) SetEnabled(on bool) { h.enabled.Store(on) }

// Enabled 当前开关状态
func (h *Hub) Enabled() bool { return h.enabled.Load() }

// Subscribe 订阅频道。超 MaxConns 返回 ErrTooManyConns
func (h *Hub) Subscribe(key string) (*Client, error) {
	h.mu.Lock()
	if int64(h.totalLocked()) >= h.maxConns.Load() {
		h.mu.Unlock()
		return nil, ErrTooManyConns
	}
	cl := &Client{
		Key:     key,
		channel: make(chan Event, ClientBufferSize),
		done:    make(chan struct{}),
	}
	if h.clients[key] == nil {
		h.clients[key] = make(map[*Client]struct{})
	}
	h.clients[key][cl] = struct{}{}
	h.mu.Unlock()
	return cl, nil
}

// Unsubscribe 退订并释放连接（关闭 done 唤醒 SSE 循环退出）
func (h *Hub) Unsubscribe(key string, cl *Client) {
	if cl == nil {
		return
	}
	h.mu.Lock()
	if set, ok := h.clients[key]; ok {
		delete(set, cl)
		if len(set) == 0 {
			delete(h.clients, key) // 频道无人订阅时回收，防 map 无限增长
		}
	}
	h.mu.Unlock()
	cl.closeDone()
}

// Publish 向频道广播一条事件。
// 事件不立即下发：先进入各订阅者的 pending 队列，
//   - pending 达到 BatchSize → 立即合并 flush（不等周期，保证低延迟观感）；
//   - 否则由 flushLoop 按 BatchIntervalSec 周期 flush。
func (h *Hub) Publish(key string, ev Event) {
	batchSize := h.batchSize.Load()
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}
	for _, cl := range h.snapshotOf(key) {
		if int64(cl.appendPending(ev)) >= batchSize {
			h.deliver(cl, Event{Name: EventNameBatch, Data: EventBatch{Events: cl.takePending()}})
		}
	}
}

// PublishBatch 批量广播（调用方已持有多条事件时逐条入队，语义同 Publish）
func (h *Hub) PublishBatch(key string, evs []Event) {
	for _, ev := range evs {
		h.Publish(key, ev)
	}
}

// deliver 投递一个事件；channel 满 → 判定慢客户端，立即断开（防内存膨胀）
func (h *Hub) deliver(cl *Client, ev Event) {
	if !cl.send(ev) {
		h.Unsubscribe(cl.Key, cl)
	}
}

// flushLoop 单线程批量 flush 线程。定时周期每次重启前重新读取配置，
// 管理后台改周期无需重启 goroutine 即生效
func (h *Hub) flushLoop() {
	for {
		interval := time.Duration(h.batchInterval.Load())
		if interval <= 0 {
			interval = DefaultBatchInterval
		}
		timer := time.NewTimer(interval)
		select {
		case <-h.stop:
			timer.Stop()
			return
		case <-timer.C:
			h.flushAll()
		}
	}
}

// flushAll 把所有客户端的 pending 合并投递（每个客户端至多一个批量信封）
func (h *Hub) flushAll() {
	for _, cl := range h.snapshotAll() {
		evs := cl.takePending()
		if len(evs) == 0 {
			continue
		}
		h.deliver(cl, Event{Name: EventNameBatch, Data: EventBatch{Events: evs}})
	}
}

// snapshotOf 取某频道的客户端快照（持锁时间最小化，投递在锁外进行）
func (h *Hub) snapshotOf(key string) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	set := h.clients[key]
	if len(set) == 0 {
		return nil
	}
	out := make([]*Client, 0, len(set))
	for cl := range set {
		out = append(out, cl)
	}
	return out
}

// snapshotAll 取全部频道客户端快照
func (h *Hub) snapshotAll() []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*Client, 0, len(h.clients))
	for _, set := range h.clients {
		for cl := range set {
			out = append(out, cl)
		}
	}
	return out
}

// totalLocked 全局连接总数（调用方持写锁）
func (h *Hub) totalLocked() int {
	n := 0
	for _, set := range h.clients {
		n += len(set)
	}
	return n
}

// Stats 运行态统计（管理后台/健康检查展示）
func (h *Hub) Stats() map[string]any {
	h.mu.RLock()
	channels := len(h.clients)
	clients := h.totalLocked()
	h.mu.RUnlock()
	return map[string]any{
		"channels":            channels,
		"clients":             clients,
		"max_conns":           int(h.maxConns.Load()),
		"batch_interval_sec":  float64(h.batchInterval.Load()) / float64(time.Second),
		"batch_size":          int(h.batchSize.Load()),
		"client_buffer":       ClientBufferSize,
		"enabled":             h.enabled.Load(),
		"heartbeat_interval_": int(HeartbeatInterval / time.Second),
	}
}

// DefaultHub 全局单例（main.go 启动即就绪，评论创建后直接广播）
var DefaultHub = NewHub()

// ChannelKey 拼装频道 key。targetType: "novel"（作品/章节评论）/ "blog"（作者博客评论）。
// 与前端约定一致：comment:novel:<novelID> / comment:blog:<blogID>
func ChannelKey(targetType string, targetID uint) string {
	return "comment:" + targetType + ":" + strconv.FormatUint(uint64(targetID), 10)
}

// PublishCommentEvent 评论创建后的便捷广播（供 handlers/comment.go 接线调用）。
// 狂暴模式关闭时短路：无订阅者时本就是空转，显式跳过省一次 map 查找，
// 同时保证管理员关闭后滞留连接也不再收到推送。
func PublishCommentEvent(targetType string, targetID uint, payload any) {
	if targetID == 0 || !DefaultHub.Enabled() {
		return
	}
	DefaultHub.Publish(ChannelKey(targetType, targetID), Event{Name: EventNameComment, Data: payload})
}
